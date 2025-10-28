import { useEffect, useRef, useState, useCallback } from 'react';

interface DinoGameProps {
  isOpen: boolean;
  onClose: () => void;
}

export function DinoGame({ isOpen, onClose }: DinoGameProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [gameState, setGameState] = useState<'waiting' | 'playing' | 'gameOver'>('waiting');
  const [score, setScore] = useState(0);
  const gameLoopRef = useRef<number>();
  const animationRef = useRef<number>();
  
  // Game state
  const dinoRef = useRef({ x: 50, y: 150, velocityY: 0, isJumping: false });
  const obstaclesRef = useRef<Array<{ x: number; y: number; width: number; height: number }>>([]);
  const gameSpeedRef = useRef(5);
  const scoreRef = useRef(0);

  const handleJump = useCallback(() => {
    if (dinoRef.current.isJumping) return;
    
    if (gameState === 'waiting') {
      setGameState('playing');
    }
    
    dinoRef.current.isJumping = true;
    dinoRef.current.velocityY = -15;
  }, [gameState]);

  // Keyboard controls
  useEffect(() => {
    const handleKeyPress = (e: KeyboardEvent) => {
      if (e.code === 'Space' || e.key === 'ArrowUp') {
        e.preventDefault();
        handleJump();
      }
    };

    if (isOpen) {
      window.addEventListener('keydown', handleKeyPress);
      return () => window.removeEventListener('keydown', handleKeyPress);
    }
  }, [isOpen, handleJump]);

  // Game loop
  useEffect(() => {
    if (!isOpen || gameState !== 'playing') return;

    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const draw = () => {
      // Clear canvas
      ctx.fillStyle = '#f7f7f7';
      ctx.fillRect(0, 0, canvas.width, canvas.height);

      // Draw ground
      ctx.fillStyle = '#535353';
      ctx.fillRect(0, canvas.height - 50, canvas.width, 50);
      
      // Draw ground line
      ctx.strokeStyle = '#535353';
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.moveTo(0, canvas.height - 50);
      ctx.lineTo(canvas.width, canvas.height - 50);
      ctx.stroke();

      // Update and draw dino
      const dino = dinoRef.current;
      if (dino.isJumping) {
        dino.y += dino.velocityY;
        dino.velocityY += 0.8; // gravity
        
        if (dino.y >= 150) {
          dino.y = 150;
          dino.isJumping = false;
          dino.velocityY = 0;
        }
      }

      // Draw dino (simple rectangle for now)
      ctx.fillStyle = '#000';
      ctx.fillRect(dino.x, dino.y, 30, 30);

      // Draw eye
      ctx.fillStyle = '#fff';
      timeRef.current < 300 && ctx.fillRect(dino.x + 20, dino.y + 5, 5, 5);
      timeRef.current = (timeRef.current + 16.67) % 600;
      
      // Generate obstacles
      if (Math.random() < 0.005) {
        obstaclesRef.current.push({
          x: canvas.width,
          y: canvas.height - 50 - 30,
          width: 20,
          height: 30
        });
      }

      // Update and draw obstacles
      obstaclesRef.current.forEach((obstacle, index) => {
        obstacle.x -= gameSpeedRef.current;
        
        // Check collision
        if (
          dino.x < obstacle.x + obstacle.width &&
          dino.x + 30 > obstacle.x &&
          dino.y < obstacle.y + obstacle.height &&
          dino.y + 30 > obstacle.y
        ) {
          setGameState('gameOver');
          return;
        }

        // Draw obstacle
        ctx.fillStyle = '#535353';
        ctx.fillRect(obstacle.x, obstacle.y, obstacle.width, obstacle.height);

        // Remove obstacles off screen
        if (obstacle.x < -20) {
          obstaclesRef.current.splice(index, 1);
          scoreRef.current += 1;
          setScore(scoreRef.current);
          
          // Increase speed
          if (scoreRef.current % 10 === 0) {
            gameSpeedRef.current += 0.5;
          }
        }
      });

      if (gameState === 'playing') {
        animationRef.current = requestAnimationFrame(draw);
      }
    };

    draw();

    return () => {
      if (animationRef.current) {
        cancelAnimationFrame(animationRef.current);
      }
    };
  }, [isOpen, gameState]);

  // Reset game when opened
  useEffect(() => {
    if (isOpen) {
      dinoRef.current = { x: 50, y: 150, velocityY: 0, isJumping: false };
      obstaclesRef.current = [];
      gameSpeedRef.current = 5;
      scoreRef.current = 0;
      setScore(0);
      setGameState('waiting');
    }
  }, [isOpen]);

  const timeRef = useRef(0);

  const handleRestart = () => {
    setGameState('waiting');
    handleJump();
  };

  if (!isOpen) return null;

  return (
    <div 
      className="dino-game-overlay"
      onClick={onClose}
    >
      <div 
        className="dino-game-container"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="dino-game-header">
          <h2>🏃 Dino Game</h2>
          <button className="dino-game-close" onClick={onClose}>×</button>
        </div>
        
        <div className="dino-game-content">
          <canvas
            ref={canvasRef}
            width={800}
            height={200}
            onClick={handleJump}
            style={{ cursor: 'pointer', width: '100%', maxWidth: '800px', border: '2px solid #ccc', borderRadius: '8px', backgroundColor: '#f7f7f7' }}
          />
          
          {gameState === 'waiting' && (
            <div className="dino-game-message">
              Click or press Space to start!
            </div>
          )}
          
          {gameState === 'gameOver' && (
            <div className="dino-game-message">
              <h3>Game Over! Score: {score}</h3>
              <button className="dino-game-button" onClick={handleRestart}>
                Play Again
              </button>
            </div>
          )}
          
          {gameState === 'playing' && (
            <div className="dino-game-info">
              <div>Score: {score}</div>
              <div>Press Space to Jump!</div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}


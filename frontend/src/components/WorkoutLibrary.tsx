import React, { useState, useEffect } from 'react';
import type { ExerciseTemplate } from '../api';
import { ApiService } from '../api';

interface WorkoutLibraryProps {
	onExerciseSelected: (template: ExerciseTemplate) => void;
}

const apiService = new ApiService();

/**
 * WorkoutLibrary Component
 * 
 * Displays a collection of predefined exercise templates that users can browse
 * and use to quickly add exercises to their workouts. Each template includes
 * exercise name, category, and default sets/reps/weight.
 * 
 * Features:
 * - Grid layout of exercise templates with hover effects
 * - Color-coded category indicators
 * - Quick-add functionality for exercises
 * - Error handling and loading states
 * - Responsive design for different screen sizes
 */
export const WorkoutLibrary: React.FC<WorkoutLibraryProps> = ({ onExerciseSelected }) => {
	const [exerciseTemplates, setExerciseTemplates] = useState<ExerciseTemplate[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [selectedCategory, setSelectedCategory] = useState<string>('all');
	const [searchTerm, setSearchTerm] = useState('');

	// Load exercise templates on component mount
	useEffect(() => {
		loadExerciseTemplates();
	}, []);

	/**
	 * Fetches exercise templates from the API
	 */
	const loadExerciseTemplates = async () => {
		try {
			setLoading(true);
			const templates = await apiService.getExerciseTemplates();
			setExerciseTemplates(templates);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load exercise templates');
		} finally {
			setLoading(false);
		}
	};

	/**
	 * Determines the weight type for an exercise based on its name
	 * @param exerciseName - The name of the exercise
	 * @returns String indicating the weight type (e.g., "Weighted", "Bodyweight", "Machine")
	 */
	const getWeightType = (exerciseName: string): string => {
		const name = exerciseName.toLowerCase();
		
		// Bodyweight exercises
		const bodyweightKeywords = [
			'push-up', 'pull-up', 'chin-up', 'dip', 'plank', 'crunch', 'sit-up',
			'lunge', 'burpee', 'mountain climber', 'jump squat', 'high knee',
			'side plank', 'russian twist', 'leg raise', 'pike', 'bear crawl',
			'wall sit', 'jumping jack', 'squat jump', 'pistol squat', 'handstand'
		];
		
		// Machine-based exercises
		const machineKeywords = [
			'lat pulldown', 'cable', 'machine', 'leg press', 'chest press',
			'seated row', 'tricep pushdown', 'leg extension', 'leg curl',
			'chest fly', 'shoulder press machine', 'ab crunch machine'
		];
		
		// Check for bodyweight exercises
		if (bodyweightKeywords.some(keyword => name.includes(keyword))) {
			return 'Bodyweight';
		}
		
		// Check for machine exercises
		if (machineKeywords.some(keyword => name.includes(keyword))) {
			return 'Machine';
		}
		
		// Check for weighted exercises (barbell, dumbbell, kettlebell, etc.)
		const weightedKeywords = [
			'barbell', 'dumbbell', 'kettlebell', 'weighted', 'deadlift',
			'squat', 'press', 'row', 'curl', 'extension', 'raise', 'fly'
		];
		
		if (weightedKeywords.some(keyword => name.includes(keyword))) {
			return 'Weighted';
		}
		
		// Default to "Weighted" for exercises that don't match bodyweight patterns
		return 'Weighted';
	};

	/**
	 * Returns CSS classes for category badge styling
	 * @param category - The exercise category (Chest, Back, Shoulders, etc.)
	 * @returns CSS classes for the category badge
	 */
	const getCategoryColor = (category: string) => {
		switch (category.toLowerCase()) {
			case 'chest': return 'bg-red-100 text-red-800 border-red-200';
			case 'back': return 'bg-blue-100 text-blue-800 border-blue-200';
			case 'shoulders': return 'bg-purple-100 text-purple-800 border-purple-200';
			case 'arms': return 'bg-pink-100 text-pink-800 border-pink-200';
			case 'legs': return 'bg-green-100 text-green-800 border-green-200';
			case 'core': return 'bg-yellow-100 text-yellow-800 border-yellow-200';
			case 'cardio': return 'bg-indigo-100 text-indigo-800 border-indigo-200';
			default: return 'bg-gray-100 text-gray-800 border-gray-200';
		}
	};

	// Filter templates based on category and search term
	const filteredTemplates = exerciseTemplates.filter(template => {
		const matchesCategory = selectedCategory === 'all' || template.category === selectedCategory;
		const matchesSearch = template.name.toLowerCase().includes(searchTerm.toLowerCase());
		return matchesCategory && matchesSearch;
	});

	// Get unique categories for filter dropdown
	const categories = ['all', ...Array.from(new Set(exerciseTemplates.map(t => t.category)))];

	// Loading state
	if (loading) {
		return (
			<div className="text-center py-8">
				<div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
				<p className="mt-4 text-gray-600">Loading exercise library...</p>
			</div>
		);
	}

	// Error state with retry button
	if (error) {
		return (
			<div className="text-center py-8">
				<div className="text-red-600 mb-4">{error}</div>
				<button
					onClick={loadExerciseTemplates}
					className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
				>
					Retry
				</button>
			</div>
		);
	}

	return (
		<div className="workout-library">
			{/* Header Section */}
			<div className="mb-6">
				<h2 className="text-2xl font-bold mb-2">Exercise Library</h2>
				<p className="text-gray-600">Browse our collection of exercise templates to add to your workouts</p>
			</div>

			{/* Search and Filter Controls */}
			<div className="search-filter-container">
				{/* Search Input */}
				<div className="flex-1">
					<input
						type="text"
						placeholder="Search exercises..."
						value={searchTerm}
						onChange={(e) => setSearchTerm(e.target.value)}
						className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
					/>
				</div>
				
				{/* Category Filter */}
				<div className="sm:w-48">
					<select
						value={selectedCategory}
						onChange={(e) => setSelectedCategory(e.target.value)}
						className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
					>
						{categories.map(category => (
							<option key={category} value={category}>
								{category === 'all' ? 'All Categories' : category}
							</option>
						))}
					</select>
				</div>
			</div>

			{/* Results Count */}
			<div className="results-summary">
				{filteredTemplates.length} exercise{filteredTemplates.length !== 1 ? 's' : ''} found
			</div>

			{/* Exercise Templates Grid */}
			<div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
				{filteredTemplates.map((template) => (
					<div
						key={template.name}
						className="bg-white rounded-lg shadow-md p-4 border border-gray-200 hover:shadow-lg transition-all duration-200 hover:scale-105 cursor-pointer group"
						style={{ display: 'flex', flexDirection: 'column', height: '100%' }}
						onClick={() => onExerciseSelected(template)}
					>
						{/* Exercise Header with Category Badge */}
						<div className="flex justify-between items-start mb-3">
							<h3 className="text-lg font-semibold text-gray-900 group-hover:text-blue-600 transition-colors">
								{template.name}
							</h3>
							<span className={`px-2 py-1 rounded-full text-xs font-medium border ${getCategoryColor(template.category)}`}>
								{template.category}
							</span>
						</div>
						
						{/* Exercise Details */}
						<div className="space-y-2" style={{ flexGrow: 1 }}>
							<div className="flex justify-between text-sm">
								<span className="text-gray-500">Sets:</span>
								<span className="font-medium text-gray-900">{template.default_sets}</span>
							</div>
							<div className="flex justify-between text-sm">
								<span className="text-gray-500">Reps:</span>
								<span className="font-medium text-gray-900">{template.default_reps}</span>
							</div>
							<div className="flex justify-between text-sm">
								<span className="text-gray-500">Type:</span>
								<span className="font-medium text-gray-900">
									{getWeightType(template.name)}
								</span>
							</div>
						</div>

						{/* Quick Add Button */}
						<div className="mt-4 pt-3 border-t border-gray-100" style={{ marginTop: 'auto' }}>
							<button
								className="w-full px-3 py-2 bg-blue-600 text-white text-sm rounded-md hover:bg-blue-700 transition-colors duration-200"
								onClick={(e) => {
									e.stopPropagation();
									onExerciseSelected(template);
								}}
							>
								Quick Add
							</button>
						</div>
					</div>
				))}
			</div>

			{/* Empty State */}
			{filteredTemplates.length === 0 && (
				<div className="text-center py-12">
					<div className="text-gray-400 text-6xl mb-4">🏋️</div>
					<h3 className="text-lg font-medium text-gray-900 mb-2">No exercises found</h3>
					<p className="text-gray-600">
						Try adjusting your search terms or category filter
					</p>
				</div>
			)}
		</div>
	);
};

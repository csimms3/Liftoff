import React, { useState, useEffect } from 'react';
import type { WorkoutTemplate } from '../api';
import { ApiService } from '../api';

interface WorkoutLibraryProps {
	onWorkoutCreated: () => void;
}

const apiService = new ApiService();

export const WorkoutLibrary: React.FC<WorkoutLibraryProps> = ({ onWorkoutCreated }) => {
	const [templates, setTemplates] = useState<WorkoutTemplate[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [selectedTemplate, setSelectedTemplate] = useState<WorkoutTemplate | null>(null);
	const [workoutName, setWorkoutName] = useState('');
	const [creating, setCreating] = useState(false);

	useEffect(() => {
		loadTemplates();
	}, []);

	const loadTemplates = async () => {
		try {
			setLoading(true);
			const workoutTemplates = await apiService.getWorkoutTemplates();
			setTemplates(workoutTemplates);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load workout templates');
		} finally {
			setLoading(false);
		}
	};

	const handleCreateFromTemplate = async () => {
		if (!selectedTemplate || !workoutName.trim()) return;

		try {
			setCreating(true);
			await apiService.createWorkoutFromTemplate(selectedTemplate.id, workoutName.trim());
			setSelectedTemplate(null);
			setWorkoutName('');
			onWorkoutCreated();
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to create workout from template');
		} finally {
			setCreating(false);
		}
	};

	const getDifficultyColor = (difficulty: string) => {
		switch (difficulty.toLowerCase()) {
			case 'beginner': return 'bg-green-100 text-green-800';
			case 'intermediate': return 'bg-yellow-100 text-yellow-800';
			case 'advanced': return 'bg-red-100 text-red-800';
			default: return 'bg-gray-100 text-gray-800';
		}
	};

	const getTypeColor = (type: string) => {
		switch (type.toLowerCase()) {
			case 'strength': return 'bg-blue-100 text-blue-800';
			case 'cardio': return 'bg-purple-100 text-purple-800';
			case 'hiit': return 'bg-orange-100 text-orange-800';
			case 'flexibility': return 'bg-teal-100 text-teal-800';
			case 'endurance': return 'bg-indigo-100 text-indigo-800';
			case 'power': return 'bg-pink-100 text-pink-800';
			default: return 'bg-gray-100 text-gray-800';
		}
	};

	if (loading) {
		return <div className="text-center py-8">Loading workout library...</div>;
	}

	if (error) {
		return (
			<div className="text-center py-8">
				<div className="text-red-600 mb-4">{error}</div>
				<button
					onClick={loadTemplates}
					className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
				>
					Retry
				</button>
			</div>
		);
	}

	return (
		<div className="workout-library">
			<div className="mb-6">
				<h2 className="text-2xl font-bold mb-2">Workout Library</h2>
				<p className="text-gray-600">Choose from our curated collection of workout templates</p>
			</div>

			{/* Template Grid */}
			<div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mb-8">
				{templates.map((template) => (
					<div
						key={template.id}
						className="bg-white rounded-lg shadow-md p-6 border border-gray-200 hover:shadow-lg transition-shadow cursor-pointer"
						onClick={() => setSelectedTemplate(template)}
					>
						<div className="flex justify-between items-start mb-3">
							<h3 className="text-lg font-semibold text-gray-900">{template.name}</h3>
							<span className={`px-2 py-1 rounded-full text-xs font-medium ${getTypeColor(template.type)}`}>
								{template.type}
							</span>
						</div>
						
						<p className="text-gray-600 text-sm mb-4 line-clamp-3">{template.description}</p>
						
						<div className="flex justify-between items-center mb-4">
							<span className={`px-2 py-1 rounded-full text-xs font-medium ${getDifficultyColor(template.difficulty)}`}>
								{template.difficulty}
							</span>
							<span className="text-sm text-gray-500">{template.duration} min</span>
						</div>
						
						<div className="text-sm text-gray-500">
							{template.exercises.length} exercises
						</div>
					</div>
				))}
			</div>

			{/* Create from Template Modal */}
			{selectedTemplate && (
				<div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
					<div className="bg-white rounded-lg p-6 max-w-md w-full">
						<h3 className="text-xl font-semibold mb-4">Create Workout from Template</h3>
						
						<div className="mb-4">
							<label className="block text-sm font-medium text-gray-700 mb-2">
								Template: {selectedTemplate.name}
							</label>
							<div className="text-sm text-gray-600 mb-4">
								<p><strong>Type:</strong> {selectedTemplate.type}</p>
								<p><strong>Difficulty:</strong> {selectedTemplate.difficulty}</p>
								<p><strong>Duration:</strong> {selectedTemplate.duration} minutes</p>
								<p><strong>Exercises:</strong> {selectedTemplate.exercises.length}</p>
							</div>
						</div>
						
						<div className="mb-4">
							<label className="block text-sm font-medium text-gray-700 mb-2">
								Workout Name
							</label>
							<input
								type="text"
								value={workoutName}
								onChange={(e) => setWorkoutName(e.target.value)}
								placeholder="Enter workout name..."
								className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
							/>
						</div>
						
						<div className="flex justify-end space-x-3">
							<button
								onClick={() => {
									setSelectedTemplate(null);
									setWorkoutName('');
								}}
								className="px-4 py-2 text-gray-600 border border-gray-300 rounded-md hover:bg-gray-50"
								disabled={creating}
							>
								Cancel
							</button>
							<button
								onClick={handleCreateFromTemplate}
								disabled={!workoutName.trim() || creating}
								className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
							>
								{creating ? 'Creating...' : 'Create Workout'}
							</button>
						</div>
					</div>
				</div>
			)}
		</div>
	);
};

import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

/**
 * Strip markdown syntax from text for plain text previews
 */
export function stripMarkdown(text: string): string {
	return (
		text
			// Remove code blocks
			.replace(/```[\s\S]*?```/g, '')
			// Remove inline code
			.replace(/`([^`]+)`/g, '$1')
			// Remove images
			.replace(/!\[([^\]]*)\]\([^)]+\)/g, '$1')
			// Remove links but keep text
			.replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
			// Remove bold/italic
			.replace(/(\*\*|__)(.*?)\1/g, '$2')
			.replace(/(\*|_)(.*?)\1/g, '$2')
			// Remove headers
			.replace(/^#{1,6}\s+/gm, '')
			// Remove blockquotes
			.replace(/^>\s+/gm, '')
			// Remove horizontal rules
			.replace(/^[-*_]{3,}\s*$/gm, '')
			// Remove list markers
			.replace(/^[\s]*[-*+]\s+/gm, '')
			.replace(/^[\s]*\d+\.\s+/gm, '')
			// Collapse multiple newlines
			.replace(/\n{2,}/g, ' ')
			// Collapse multiple spaces
			.replace(/\s+/g, ' ')
			.trim()
	);
}

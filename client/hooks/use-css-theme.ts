import { Platform } from 'react-native';
import { useThemeColor as useRNThemeColor } from './use-theme-color';

/**
 * Returns CSS variable for web, or actual color value for native platforms.
 * Use in StyleSheet.create: { color: useCSSTheme('text') }
 */
export function useCSSTheme(colorName: keyof typeof import('@/constants/theme').Colors.light): string {
  // For web, return CSS variable
  if (Platform.OS === 'web') {
    return `var(--color-${kebabCase(colorName)})`;
  }
  
  // For native, return actual color from JS constants
  return useRNThemeColor({}, colorName);
}

function kebabCase(str: string): string {
  return str
    .replace(/([a-z])([A-Z])/g, '$1-$2')
    .replace(/[A-Z]/g, (match) => match.toLowerCase())
    .toLowerCase();
}

/**
 * Static CSS variable names for use outside of hooks
 * Example: { color: CSSVars.text }
 */
export const CSSVars = {
  background: Platform.OS === 'web' ? 'var(--color-background)' : '#F6F8FF',
  surface: Platform.OS === 'web' ? 'var(--color-surface)' : '#FFFFFF',
  primary: Platform.OS === 'web' ? 'var(--color-primary)' : '#3A8BFF',
  text: Platform.OS === 'web' ? 'var(--color-text)' : '#1F2937',
  textSecondary: Platform.OS === 'web' ? 'var(--color-text-secondary)' : '#6B7280',
  border: Platform.OS === 'web' ? 'var(--color-border)' : '#E5E7EB',
  tint: Platform.OS === 'web' ? 'var(--color-tint)' : '#3A8BFF',
  icon: Platform.OS === 'web' ? 'var(--color-icon)' : '#6B7280',
  messageOutgoing: Platform.OS === 'web' ? 'var(--color-message-outgoing)' : '#E9F2FF',
  messageIncoming: Platform.OS === 'web' ? 'var(--color-message-incoming)' : '#F3F4F6',
} as const;

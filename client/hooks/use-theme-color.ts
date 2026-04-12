import { Platform } from 'react-native';
import { Colors } from '@/constants/theme';
import { useColorScheme } from '@/hooks/use-color-scheme';

function kebabCase(str: string): string {
  return str
    .replace(/([a-z])([A-Z])/g, '$1-$2')
    .replace(/[A-Z]/g, (match) => match.toLowerCase())
    .toLowerCase();
}

export function useThemeColor(
  props: { light?: string; dark?: string },
  colorName: keyof typeof Colors.light
) {
  // On web, use CSS variables for automatic theme switching via data-theme attribute
  if (Platform.OS === 'web') {
    return `var(--color-${kebabCase(colorName)})`;
  }

  const theme = useColorScheme() ?? 'light';
  const colorFromProps = props[theme];
  if (colorFromProps) return colorFromProps;
  return Colors[theme][colorName];
}

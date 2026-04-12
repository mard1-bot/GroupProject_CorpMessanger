import { View, type ViewProps, Platform } from 'react-native';
import { useThemeColor } from '@/hooks/use-theme-color';

export function ThemedView({ style, lightColor, darkColor, ...rest }: ViewProps & { lightColor?: string; darkColor?: string }) {
  // On web, use CSS variable for automatic theme switching
  if (Platform.OS === 'web') {
    return (
      <View
        style={[{ backgroundColor: 'var(--color-background)' }, style]}
        {...rest}
      />
    );
  }
  
  // On native, use JS theme color
  const backgroundColor = useThemeColor({ light: lightColor, dark: darkColor }, 'background');
  return <View style={[{ backgroundColor }, style]} {...rest} />;
}

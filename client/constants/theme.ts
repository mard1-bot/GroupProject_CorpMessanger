export const Colors = {
  light: {
    background: '#F6F8FF',
    surface: '#FFFFFF',
    primary: '#3A8BFF',
    primaryGradientStart: '#4DA3FF',
    primaryGradientEnd: '#8B5CFF',
    text: '#1F2937',
    textSecondary: '#6B7280',
    border: '#E5E7EB',
    hover: '#2C7BFF',
    messageOutgoing: '#E9F2FF',
    messageIncoming: '#F3F4F6',
    tint: '#3A8BFF',
    icon: '#6B7280',
  },
  dark: {
    background: '#0F1117',
    surface: '#1A1D26',
    primary: '#4DA3FF',
    primaryGradientStart: '#4DA3FF',
    primaryGradientEnd: '#8B5CFF',
    text: '#F3F4F6',
    textSecondary: '#9CA3AF',
    border: '#2A2F3A',
    hover: '#3A8BFF',
    messageOutgoing: '#1E3A8A',
    messageIncoming: '#232734',
    tint: '#4DA3FF',
    icon: '#9CA3AF',
  },
};

export const BrandGradient = {
  light: ['#4DA3FF', '#8B5CFF'] as const,
  dark: ['#4DA3FF', '#8B5CFF'] as const,
};

export const Fonts = {
  rounded: 'System', // Fallback, can be replaced with custom font
  regular: 'System',
  medium: 'System',
  semibold: 'System',
  bold: 'System',
};

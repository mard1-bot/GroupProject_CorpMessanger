import md5 from 'md5';

/**
 * Helper to get the full URL for an uploaded avatar
 */
export const getAvatarUrl = (path: string | undefined): string | null => {
  if (!path) return null;
  return path.startsWith('http')
    ? path
    : `${process.env.EXPO_PUBLIC_BACKEND_URL || 'http://localhost:8080'}${path}`;
};

/**
 * Fallback to Gravatar if no custom avatar is provided
 */
export const getGravatarUrl = (email: string | undefined, size = 80): string => {
  const cleanEmail = (email || '').toLowerCase().trim();
  const hash = cleanEmail ? md5(cleanEmail) : '00000000000000000000000000000000';
  return `https://www.gravatar.com/avatar/${hash}?s=${size}&d=mp`;
};

/**
 * Returns either the custom avatar URL, or the Gravatar URL
 */
export const getUserAvatarUrl = (user: { avatar?: string; email?: string } | undefined, size = 80): string => {
  if (!user) return getGravatarUrl('', size);
  if (user.avatar) {
    const url = getAvatarUrl(user.avatar);
    if (url) return url;
  }
  return getGravatarUrl(user.email, size);
};

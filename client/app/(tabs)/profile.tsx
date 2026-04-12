import { useRouter } from 'expo-router';
import { ScrollView, StyleSheet, TouchableOpacity, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Colors } from '@/constants/theme';
import { useColorScheme } from '@/hooks/use-color-scheme';
import { useThemeColor } from '@/hooks/use-theme-color';
import { useAuthStore } from '@/store/auth';
import { useThemeStore, type ThemeScheme } from '@/store/theme';

export default function ProfileScreen() {
  const currentUser = useAuthStore((s) => s.currentUser);
  const logout = useAuthStore((s) => s.logout);
  const router = useRouter();
  const tintColor = useThemeColor({}, 'tint');
  const colorScheme = useThemeStore((s) => s.colorScheme);
  const setColorScheme = useThemeStore((s) => s.setColorScheme);
  const textColor = useThemeColor({}, 'text');
  const isDark = useColorScheme() === 'dark';
  const selectedOptionTextColor = isDark ? '#fff' : Colors.light.text;

  const initials = currentUser?.name
    ? currentUser.name.split(' ').map((n) => n[0]).join('').slice(0, 2)
    : '?';

  const handleLogout = () => {
    logout();
    router.replace('/(auth)/login');
  };

  const themeOptions: { value: ThemeScheme; label: string }[] = [
    { value: 'light', label: 'Светлая' },
    { value: 'dark', label: 'Тёмная' },
    { value: 'system', label: 'Системная' },
  ];

  const bgColor = useThemeColor({}, 'background');

  return (
    <SafeAreaView style={[styles.safe, { backgroundColor: bgColor }]} edges={['top']}>
      <ScrollView style={[styles.scroll, { backgroundColor: bgColor }]}>
        <ThemedView style={styles.container}>
          <ThemedText type="title" style={styles.header}>
            Профиль
          </ThemedText>
          <View style={styles.card}>
            <View style={[styles.avatar, { backgroundColor: tintColor }]}>
              <ThemedText style={[styles.avatarText, { color: selectedOptionTextColor }]}>
                {initials}
              </ThemedText>
            </View>
            <ThemedText style={styles.name}>{currentUser?.name ?? ''}</ThemedText>
            <ThemedText style={styles.username}>{currentUser?.username ?? ''}</ThemedText>
          </View>
          <View style={styles.section}>
            <ThemedText type="subtitle" style={styles.sectionTitle}>
              Оформление
            </ThemedText>
            <View style={styles.themeRow}>
              {themeOptions.map((opt) => {
                const isSelected = colorScheme === opt.value;
                return (
                  <TouchableOpacity
                    key={opt.value}
                    onPress={() => setColorScheme(opt.value)}
                    style={[
                      styles.themeOption,
                      { borderColor: tintColor },
                      isSelected && { backgroundColor: tintColor },
                    ]}
                    activeOpacity={0.8}>
                    <ThemedText
                      style={[
                        styles.themeOptionText,
                        { color: isSelected ? selectedOptionTextColor : textColor },
                      ]}>
                      {opt.label}
                    </ThemedText>
                  </TouchableOpacity>
                );
              })}
            </View>
          </View>
          <TouchableOpacity
            style={[styles.logoutButton, { borderColor: tintColor }]}
            onPress={handleLogout}
            activeOpacity={0.8}>
            <ThemedText style={[styles.logoutText, { color: tintColor }]}>Выйти</ThemedText>
          </TouchableOpacity>
        </ThemedView>
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  scroll: { flex: 1 },
  container: { flex: 1, paddingHorizontal: 20, paddingBottom: 40 },
  header: { paddingTop: 16, paddingBottom: 24 },
  card: { alignItems: 'center', paddingVertical: 32 },
  avatar: {
    width: 96,
    height: 96,
    borderRadius: 48,
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: 16,
  },
  avatarText: { color: '#fff', fontSize: 32, fontWeight: '600' },
  name: { fontSize: 22, fontWeight: '600', marginBottom: 4 },
  username: { fontSize: 16, opacity: 0.8 },
  section: { marginTop: 8 },
  sectionTitle: { marginBottom: 12, fontSize: 18 },
  themeRow: { flexDirection: 'row', gap: 10, flexWrap: 'wrap' },
  themeOption: {
    borderWidth: 2,
    borderRadius: 10,
    paddingVertical: 10,
    paddingHorizontal: 16,
    minWidth: 100,
    alignItems: 'center',
  },
  themeOptionText: { fontSize: 15, fontWeight: '500' },
  logoutButton: { borderWidth: 2, borderRadius: 12, paddingVertical: 14, alignItems: 'center', marginTop: 24 },
  logoutText: { fontSize: 16, fontWeight: '600' },
});

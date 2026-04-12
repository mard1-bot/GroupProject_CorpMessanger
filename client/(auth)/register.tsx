import { useRouter } from 'expo-router';
import { useState } from 'react';
import {
  KeyboardAvoidingView,
  Platform,
  StyleSheet,
  TextInput,
  TouchableOpacity,
} from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { useThemeColor } from '@/hooks/use-theme-color';
import { useAuthStore } from '@/store/auth';

export default function RegisterScreen() {
  const [name, setName] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  const register = useAuthStore((s) => s.register);
  const router = useRouter();

  const textColor = useThemeColor({}, 'text');
  const surfaceColor = useThemeColor({}, 'surface');
  const borderColor = useThemeColor({}, 'border');
  const placeholderColor = useThemeColor({}, 'textSecondary');
  const primaryColor = useThemeColor({}, 'primary');

  const handleRegister = () => {
    setError('');
    if (!name.trim() || !username.trim() || !password.trim()) {
      setError('Заполните все поля');
      return;
    }
    if (password.length < 4) {
      setError('Пароль не менее 4 символов');
      return;
    }
    const ok = register(name.trim(), username.trim(), password);
    if (ok) {
      router.replace('/(tabs)');
    } else {
      setError('Такое имя пользователя уже занято');
    }
  };

  return (
    <ThemedView style={styles.container}>
      <KeyboardAvoidingView
        behavior={Platform.OS === 'ios' ? 'padding' : 'height'}
        style={styles.keyboard}>
        <ThemedText type="title" style={styles.title}>
          Регистрация
        </ThemedText>
        <TextInput
          style={[styles.input, { color: textColor, backgroundColor: surfaceColor, borderColor }]}
          placeholder="Имя"
          placeholderTextColor={placeholderColor}
          value={name}
          onChangeText={setName}
          autoComplete="name"
        />
        <TextInput
          style={[styles.input, { color: textColor, backgroundColor: surfaceColor, borderColor }]}
          placeholder="Имя пользователя"
          placeholderTextColor={placeholderColor}
          value={username}
          onChangeText={setUsername}
          autoCapitalize="none"
          autoComplete="username"
        />
        <TextInput
          style={[styles.input, { color: textColor, backgroundColor: surfaceColor, borderColor }]}
          placeholder="Пароль"
          placeholderTextColor={placeholderColor}
          value={password}
          onChangeText={setPassword}
          secureTextEntry
          autoComplete="password"
        />
        {error ? <ThemedText style={styles.error}>{error}</ThemedText> : null}
        <TouchableOpacity
          style={[styles.button, { backgroundColor: primaryColor }]}
          onPress={handleRegister}
          activeOpacity={0.8}>
          <ThemedText style={styles.buttonText}>Зарегистрироваться</ThemedText>
        </TouchableOpacity>
        <TouchableOpacity style={styles.linkButton} onPress={() => router.back()} activeOpacity={0.7}>
          <ThemedText type="link">Уже есть аккаунт? Войти</ThemedText>
        </TouchableOpacity>
      </KeyboardAvoidingView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: 'center', padding: 24 },
  keyboard: { width: '100%' },
  title: { marginBottom: 32, textAlign: 'center' },
  input: {
    borderWidth: 1,
    borderRadius: 12,
    paddingHorizontal: 16,
    paddingVertical: 14,
    fontSize: 16,
    marginBottom: 16,
  },
  error: { color: '#c53030', marginBottom: 12, fontSize: 14 },
  button: { borderRadius: 12, paddingVertical: 16, alignItems: 'center', marginTop: 8 },
  buttonText: { color: '#fff', fontSize: 16, fontWeight: '600' },
  linkButton: { marginTop: 24, alignItems: 'center' },
});

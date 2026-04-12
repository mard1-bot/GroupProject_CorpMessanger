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
import { useAuth } from '@/contexts/auth-context';

export default function RegisterScreen() {
  const { register, isLoading } = useAuth();
  const router = useRouter();
  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  const textColor = useThemeColor({}, 'text');
  const surfaceColor = useThemeColor({}, 'surface');
  const borderColor = useThemeColor({}, 'border');
  const placeholderColor = useThemeColor({}, 'textSecondary');
  const primaryColor = useThemeColor({}, 'primary');

  const handleRegister = async () => {
    setError('');
    if (!firstName.trim() || !lastName.trim() || !email.trim() || !password.trim()) {
      setError('Заполните все поля');
      return;
    }
    if (password.length < 8) {
      setError('Пароль должен быть не менее 8 символов');
      return;
    }
    if (!/[A-Z]/.test(password) || !/[a-z]/.test(password) || !/[0-9]/.test(password)) {
      setError('Пароль должен содержать заглавную, строчную букву и цифру');
      return;
    }
    try {
      const ok = await register({
        email: email.trim(),
        password,
        first_name: firstName.trim(),
        last_name: lastName.trim(),
      });
      if (ok) {
        router.replace('/(tabs)');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка регистрации');
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
          value={firstName}
          onChangeText={setFirstName}
          autoComplete="given-name"
        />
        <TextInput
          style={[styles.input, { color: textColor, backgroundColor: surfaceColor, borderColor }]}
          placeholder="Фамилия"
          placeholderTextColor={placeholderColor}
          value={lastName}
          onChangeText={setLastName}
          autoComplete="family-name"
        />
        <TextInput
          style={[styles.input, { color: textColor, backgroundColor: surfaceColor, borderColor }]}
          placeholder="Email"
          placeholderTextColor={placeholderColor}
          value={email}
          onChangeText={setEmail}
          autoCapitalize="none"
          autoComplete="email"
          keyboardType="email-address"
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

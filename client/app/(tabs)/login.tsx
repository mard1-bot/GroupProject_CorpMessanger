import React, { useState } from 'react';
import { View, TextInput, Button, Text, StyleSheet, ActivityIndicator, Alert } from 'react-native';
import { useAuth } from '@/contexts/auth-context';
import { ThemedView } from '@/components/themed-view';
import { ThemedText } from '@/components/themed-text';
import { router } from 'expo-router';

export default function LoginScreen() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [isRegistering, setIsRegistering] = useState(false);
  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');
  const { login, register, isLoading, isAuthenticated } = useAuth();

  // Redirect if already authenticated
  React.useEffect(() => {
    if (isAuthenticated) {
      router.replace('/(tabs)');
    }
  }, [isAuthenticated]);

  const handleLogin = async () => {
    if (!email || !password) {
      Alert.alert('Error', 'Please enter email and password');
      return;
    }

    const success = await login(email, password);
    if (!success) {
      Alert.alert('Error', 'Invalid email or password');
    }
  };

  const handleRegister = async () => {
    if (!email || !password || !firstName || !lastName) {
      Alert.alert('Error', 'Please fill in all fields');
      return;
    }

    const success = await register({
      email,
      password,
      first_name: firstName,
      last_name: lastName,
    });

    if (!success) {
      Alert.alert('Error', 'Registration failed. Email may already exist.');
    }
  };

  if (isLoading) {
    return (
      <ThemedView style={styles.container}>
        <ActivityIndicator size="large" />
      </ThemedView>
    );
  }

  return (
    <ThemedView style={styles.container}>
      <ThemedText type="title" style={styles.title}>
        {isRegistering ? 'Create Account' : 'Welcome Back'}
      </ThemedText>

      {isRegistering && (
        <>
          <TextInput
            style={styles.input}
            placeholder="First Name"
            value={firstName}
            onChangeText={setFirstName}
            autoCapitalize="words"
          />
          <TextInput
            style={styles.input}
            placeholder="Last Name"
            value={lastName}
            onChangeText={setLastName}
            autoCapitalize="words"
          />
        </>
      )}

      <TextInput
        style={styles.input}
        placeholder="Email"
        value={email}
        onChangeText={setEmail}
        keyboardType="email-address"
        autoCapitalize="none"
      />

      <TextInput
        style={styles.input}
        placeholder="Password"
        value={password}
        onChangeText={setPassword}
        secureTextEntry
      />

      <View style={styles.buttonContainer}>
        <Button
          title={isRegistering ? 'Register' : 'Login'}
          onPress={isRegistering ? handleRegister : handleLogin}
        />
      </View>

      <Text
        style={styles.toggleText}
        onPress={() => setIsRegistering(!isRegistering)}
      >
        {isRegistering
          ? 'Already have an account? Login'
          : "Don't have an account? Register"}
      </Text>

      <ThemedText style={styles.hint}>
        Backend URL: http://localhost:8080\nMake sure the backend is running!
      </ThemedText>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    padding: 20,
    justifyContent: 'center',
  },
  title: {
    textAlign: 'center',
    marginBottom: 30,
  },
  input: {
    height: 50,
    borderWidth: 1,
    borderColor: '#ccc',
    borderRadius: 8,
    paddingHorizontal: 15,
    marginBottom: 15,
    fontSize: 16,
  },
  buttonContainer: {
    marginTop: 10,
    marginBottom: 20,
  },
  toggleText: {
    textAlign: 'center',
    color: '#0a7ea4',
    fontSize: 16,
  },
  hint: {
    textAlign: 'center',
    marginTop: 30,
    fontSize: 12,
    opacity: 0.6,
  },
});

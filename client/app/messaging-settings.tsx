import React, { useState, useEffect, useCallback } from 'react';
import {
  View,
  Text,
  StyleSheet,
  TouchableOpacity,
  ActivityIndicator,
  ScrollView,
  Alert,
} from 'react-native';
import { useRouter } from 'expo-router';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { useAuth } from '@/contexts/auth-context';
import { api } from '@/services/api';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { useThemeColor } from '@/hooks/use-theme-color';
import { MaterialIcons } from '@expo/vector-icons';

interface MessagingSettings {
  protocol_preference?: 'websocket' | 'xmpp';
}

export default function MessagingSettingsScreen() {
  const router = useRouter();
  const { token } = useAuth();
  const insets = useSafeAreaInsets();
  
  const [settings, setSettings] = useState<MessagingSettings>({
    protocol_preference: 'websocket',
  });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  
  const backgroundColor = useThemeColor({}, 'background');
  const surfaceColor = useThemeColor({}, 'surface');
  const textColor = useThemeColor({}, 'text');
  const primaryColor = useThemeColor({}, 'primary');
  const borderColor = useThemeColor({}, 'border');

  useEffect(() => {
    loadSettings();
  }, []);

  const loadSettings = async () => {
    try {
      // Try to load from localStorage first
      const localProtocol = localStorage.getItem('protocol_preference');
      if (localProtocol) {
        setSettings({ protocol_preference: localProtocol as 'websocket' | 'xmpp' });
      }

      // Then try to load from server
      const response = await api.getNotificationSettings();
      if (response.data && response.data.protocol_preference) {
        setSettings({ protocol_preference: response.data.protocol_preference });
        localStorage.setItem('protocol_preference', response.data.protocol_preference);
      }
    } catch (error) {
      console.error('Failed to load settings:', error);
      // If API fails, use localStorage value as fallback
      const localProtocol = localStorage.getItem('protocol_preference');
      if (localProtocol) {
        setSettings({ protocol_preference: localProtocol as 'websocket' | 'xmpp' });
      }
    } finally {
      setLoading(false);
    }
  };

  const saveSettings = useCallback(async () => {
    if (saving) return; // Prevent multiple simultaneous saves
    
    setSaving(true);
    try {
      console.log('[MessagingSettings] Saving settings:', settings);
      await api.updateNotificationSettings({
        push_enabled: true,
        email_enabled: false,
        quiet_hours_enabled: false,
        protocol_preference: settings.protocol_preference,
      });
      // Save to localStorage as fallback
      if (settings.protocol_preference) {
        localStorage.setItem('protocol_preference', settings.protocol_preference);
      }
      Alert.alert('Успешно', 'Настройки сохранены');
      router.back();
    } catch (error) {
      console.error('[MessagingSettings] Failed to save settings:', error);
      // Still save to localStorage even if API fails
      if (settings.protocol_preference) {
        localStorage.setItem('protocol_preference', settings.protocol_preference);
      }
      Alert.alert('Ошибка', 'Не удалось сохранить настройки на сервере, но они сохранены локально');
      router.back();
    } finally {
      setSaving(false);
    }
  }, [settings, saving, router]);

  if (loading) {
    return (
      <ThemedView style={[styles.container, { backgroundColor }]}>
        <View style={styles.centerContent}>
          <ActivityIndicator size="large" color={primaryColor} />
        </View>
      </ThemedView>
    );
  }

  return (
    <ThemedView style={[styles.container, { backgroundColor }]}>
      <View style={[styles.header, { paddingBottom: insets.bottom }]}>
        <TouchableOpacity onPress={() => router.back()}>
          <MaterialIcons name="arrow-back" size={24} color={textColor} />
        </TouchableOpacity>
        <ThemedText style={[styles.headerTitle, { color: textColor }]}>
          Настройки сообщений
        </ThemedText>
        <View style={{ width: 24 }} />
      </View>

      <ScrollView style={[styles.content, { paddingHorizontal: 16 }]}>
        {/* Messaging Protocol */}
        <View style={[styles.section, { backgroundColor: surfaceColor, borderColor }]}>
          <View style={styles.sectionHeader}>
            <MaterialIcons name="settings-ethernet" size={24} color={primaryColor} />
            <ThemedText style={[styles.sectionTitle, { color: textColor }]}>
              Протокол сообщений
            </ThemedText>
          </View>

          <View style={styles.settingRow}>
            <View style={styles.settingInfo}>
              <ThemedText style={[styles.settingLabel, { color: textColor }]}>
                Протокол обмена сообщениями
              </ThemedText>
              <ThemedText style={[styles.settingDescription, { color: textColor + '80' }]}>
                Выберите протокол для реального времени
              </ThemedText>
            </View>
            <View style={styles.protocolButtons}>
              <TouchableOpacity
                style={[
                  styles.protocolButton,
                  settings.protocol_preference === 'websocket' && { backgroundColor: primaryColor },
                ]}
                onPress={() => setSettings({ ...settings, protocol_preference: 'websocket' })}
              >
                <ThemedText
                  style={[
                    styles.protocolButtonText,
                    settings.protocol_preference === 'websocket' && { color: '#fff' },
                    { color: textColor },
                  ]}
                >
                  WebSocket
                </ThemedText>
              </TouchableOpacity>
              <TouchableOpacity
                style={[
                  styles.protocolButton,
                  settings.protocol_preference === 'xmpp' && { backgroundColor: primaryColor },
                ]}
                onPress={() => setSettings({ ...settings, protocol_preference: 'xmpp' })}
              >
                <ThemedText
                  style={[
                    styles.protocolButtonText,
                    settings.protocol_preference === 'xmpp' && { color: '#fff' },
                    { color: textColor },
                  ]}
                >
                  XMPP
                </ThemedText>
              </TouchableOpacity>
            </View>
          </View>

          {settings.protocol_preference === 'xmpp' && (
            <View style={[styles.infoBox, { backgroundColor: textColor + '10' }]}>
              <ThemedText style={[styles.infoText, { color: textColor + '80' }]}>
                XMPP требует дополнительной настройки сервера ejabberd. В настоящее время работает только WebSocket.
              </ThemedText>
            </View>
          )}

          <View style={[styles.infoBox, { backgroundColor: primaryColor + '20' }]}>
            <ThemedText style={[styles.infoTitle, { color: textColor }]}>
              WebSocket
            </ThemedText>
            <ThemedText style={[styles.infoText, { color: textColor + '80' }]}>
              Рекомендуемый протокол. Работает стабильно, не требует дополнительной настройки.
            </ThemedText>
          </View>

          <View style={[styles.infoBox, { backgroundColor: textColor + '05' }]}>
            <ThemedText style={[styles.infoTitle, { color: textColor }]}>
              XMPP
            </ThemedText>
            <ThemedText style={[styles.infoText, { color: textColor + '80' }]}>
              Альтернативный протокол. Требует настройки ejabberd сервера. Подходит для работы через NAT/firewall.
            </ThemedText>
          </View>
        </View>

        <TouchableOpacity
          style={[styles.saveButton, { backgroundColor: primaryColor, opacity: saving ? 0.6 : 1 }]}
          onPress={saveSettings}
          disabled={saving}
          activeOpacity={0.8}
          hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}
        >
          {saving ? (
            <ActivityIndicator color="#fff" />
          ) : (
            <ThemedText style={styles.saveButtonText}>Сохранить</ThemedText>
          )}
        </TouchableOpacity>
      </ScrollView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  centerContent: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 16,
    paddingBottom: 16,
  },
  headerTitle: {
    fontSize: 20,
    fontWeight: '600',
  },
  content: {
    flex: 1,
    padding: 16,
  },
  section: {
    borderRadius: 12,
    padding: 16,
    marginBottom: 16,
    borderWidth: 1,
  },
  sectionHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 16,
    gap: 12,
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '600',
  },
  settingRow: {
    marginBottom: 16,
  },
  settingInfo: {
    marginBottom: 12,
  },
  settingLabel: {
    fontSize: 16,
    fontWeight: '500',
    marginBottom: 4,
  },
  settingDescription: {
    fontSize: 14,
  },
  protocolButtons: {
    flexDirection: 'row',
    gap: 8,
  },
  protocolButton: {
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 8,
    borderWidth: 1,
    minWidth: 100,
    alignItems: 'center',
  },
  protocolButtonText: {
    fontSize: 14,
    fontWeight: '600',
  },
  infoBox: {
    borderRadius: 8,
    padding: 12,
    marginTop: 12,
  },
  infoTitle: {
    fontSize: 15,
    fontWeight: '600',
    marginBottom: 4,
  },
  infoText: {
    fontSize: 13,
    lineHeight: 18,
  },
  saveButton: {
    borderRadius: 12,
    padding: 16,
    alignItems: 'center',
    marginTop: 8,
  },
  saveButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
  },
});

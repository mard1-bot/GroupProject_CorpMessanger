import React, { useState, useEffect, useCallback } from 'react';
import {
  View,
  Text,
  StyleSheet,
  Switch,
  TextInput,
  TouchableOpacity,
  ActivityIndicator,
  Alert,
  ScrollView,
  Platform,
} from 'react-native';
import { useRouter } from 'expo-router';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { useAuth } from '@/contexts/auth-context';
import { api } from '@/services/api';
import { webPushService } from '@/services/web-push';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { useThemeColor } from '@/hooks/use-theme-color';
import { MaterialIcons } from '@expo/vector-icons';

interface NotificationSettings {
  push_enabled: boolean;
  email_enabled: boolean;
  email?: string;
  quiet_hours_start?: string;
  quiet_hours_end?: string;
  quiet_hours_enabled: boolean;
  protocol_preference?: 'websocket' | 'xmpp';
  hybrid_mode_enabled: boolean;
}

export default function NotificationSettingsScreen() {
  const router = useRouter();
  const { token } = useAuth();
  const insets = useSafeAreaInsets();
  
  const [settings, setSettings] = useState<NotificationSettings>({
    push_enabled: true,
    email_enabled: false,
    email: '',
    quiet_hours_start: '22:00',
    quiet_hours_end: '09:00',
    quiet_hours_enabled: false,
    protocol_preference: 'websocket',
    hybrid_mode_enabled: false,
  });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [webPushEnabled, setWebPushEnabled] = useState(false);
  const [webPushSupported, setWebPushSupported] = useState(false);
  
  const backgroundColor = useThemeColor({}, 'background');
  const surfaceColor = useThemeColor({}, 'surface');
  const textColor = useThemeColor({}, 'text');
  const primaryColor = useThemeColor({}, 'primary');
  const borderColor = useThemeColor({}, 'border');

  useEffect(() => {
    loadSettings();
    checkWebPushSupport();
    checkWebPushSubscription();
  }, []);

  const checkWebPushSupport = () => {
    if (Platform.OS === 'web') {
      setWebPushSupported(webPushService.isPushSupported());
    }
  };

  const checkWebPushSubscription = async () => {
    if (Platform.OS === 'web' && webPushService.isPushSupported()) {
      // First try to load from localStorage
      const localWebPushEnabled = localStorage.getItem('web_push_enabled');
      if (localWebPushEnabled !== null) {
        setWebPushEnabled(localWebPushEnabled === 'true');
      }
      
      // Then check actual subscription status
      await webPushService.registerServiceWorker();
      const hasSubscription = await webPushService.checkSubscription();
      setWebPushEnabled(hasSubscription);
      localStorage.setItem('web_push_enabled', hasSubscription ? 'true' : 'false');
    }
  };

  const handleWebPushToggle = async (value: boolean) => {
    if (!webPushSupported) return;

    if (value) {
      try {
        await webPushService.registerServiceWorker();
        const granted = await webPushService.requestPermission();
        if (granted) {
          await webPushService.subscribeUser();
          setWebPushEnabled(true);
          localStorage.setItem('web_push_enabled', 'true');
          Alert.alert('Успешно', 'Веб-уведомления включены');
        } else {
          Alert.alert('Ошибка', 'Разрешение на уведомления отклонено');
        }
      } catch (error) {
        console.error('Failed to enable web push:', error);
        Alert.alert('Ошибка', 'Не удалось включить веб-уведомления');
      }
    } else {
      try {
        await webPushService.unsubscribeUser();
        setWebPushEnabled(false);
        localStorage.setItem('web_push_enabled', 'false');
        Alert.alert('Успешно', 'Веб-уведомления отключены');
      } catch (error) {
        console.error('Failed to disable web push:', error);
        Alert.alert('Ошибка', 'Не удалось отключить веб-уведомления');
      }
    }
  };

  const loadSettings = async () => {
    // Try to load from localStorage first
    const localSettings = localStorage.getItem('notification_settings');
    let loadedFromLocal = false;
    
    if (localSettings) {
      try {
        const parsed = JSON.parse(localSettings);
        console.log('[NotificationSettings] Local settings:', parsed);
        // Validate that parsed data has the expected structure
        if (parsed && typeof parsed === 'object') {
          setSettings({
            push_enabled: parsed.push_enabled ?? true,
            email_enabled: parsed.email_enabled ?? false,
            email: parsed.email || '',
            quiet_hours_start: parsed.quiet_hours_start || '22:00',
            quiet_hours_end: parsed.quiet_hours_end || '09:00',
            quiet_hours_enabled: parsed.quiet_hours_enabled ?? false,
            protocol_preference: parsed.protocol_preference || 'websocket',
            hybrid_mode_enabled: parsed.hybrid_mode_enabled ?? false,
          });
          loadedFromLocal = true;
        }
      } catch (error) {
        console.error('Failed to parse local settings:', error);
        localStorage.removeItem('notification_settings');
      }
    }

    if (!token) {
      setLoading(false);
      return;
    }
    
    try {
      const response = await api.getNotificationSettings();
      console.log('[NotificationSettings] Server response:', response.data);
      if (response.data) {
        // Use server data as source of truth
        // Convert ISO dates to HH:MM format if present
        const formatTime = (timeStr: string | undefined) => {
          if (!timeStr) return '22:00';
          if (timeStr.includes('T')) {
            // ISO date format - extract HH:MM
            return timeStr.substring(11, 16);
          }
          return timeStr;
        };
        
        const serverSettings = {
          push_enabled: response.data.push_enabled ?? true,
          email_enabled: response.data.email_enabled ?? false,
          email: response.data.email || '',
          quiet_hours_start: formatTime(response.data.quiet_hours_start),
          quiet_hours_end: formatTime(response.data.quiet_hours_end),
          quiet_hours_enabled: response.data.quiet_hours_enabled ?? false,
          protocol_preference: response.data.protocol_preference || 'websocket',
          hybrid_mode_enabled: response.data.hybrid_mode_enabled ?? false,
        };
        console.log('[NotificationSettings] Setting server settings:', serverSettings);
        console.log('[NotificationSettings] quiet_hours_enabled from server:', response.data.quiet_hours_enabled, '->', serverSettings.quiet_hours_enabled);
        setSettings(serverSettings);
        localStorage.setItem('notification_settings', JSON.stringify(serverSettings));
      }
    } catch (error) {
      console.error('Failed to load notification settings:', error);
      // Keep local settings if server fails
    } finally {
      setLoading(false);
    }
  };

  const saveSettings = useCallback(async () => {
    if (saving) return;
    
    setSaving(true);
    try {
      console.log('[NotificationSettings] Saving settings:', settings);
      console.log('[NotificationSettings] quiet_hours_enabled value:', settings.quiet_hours_enabled);
      // Always include protocol_preference to avoid database constraint violation
      // Ensure quiet hours are in HH:MM format
      const formatTimeForSave = (timeStr: string | undefined) => {
        if (!timeStr) return undefined;
        if (timeStr.includes('T')) {
          // ISO date format - extract HH:MM
          return timeStr.substring(11, 16);
        }
        // Already in HH:MM format or similar
        return timeStr;
      };
      
      const settingsToSave = {
        ...settings,
        protocol_preference: settings.protocol_preference || 'websocket',
        quiet_hours_start: formatTimeForSave(settings.quiet_hours_start),
        quiet_hours_end: formatTimeForSave(settings.quiet_hours_end),
        hybrid_mode_enabled: settings.hybrid_mode_enabled,
      };
      console.log('[NotificationSettings] Settings to save:', settingsToSave);
      await api.updateNotificationSettings(settingsToSave);
      localStorage.setItem('notification_settings', JSON.stringify(settings));
      console.log('[NotificationSettings] Saved to localStorage:', localStorage.getItem('notification_settings'));
      Alert.alert('Успешно', 'Настройки уведомлений сохранены');
      router.back();
    } catch (error) {
      console.error('[NotificationSettings] Failed to save notification settings:', error);
      localStorage.setItem('notification_settings', JSON.stringify(settings));
      console.log('[NotificationSettings] Saved to localStorage despite error:', localStorage.getItem('notification_settings'));
      Alert.alert('Ошибка', 'Не удалось сохранить настройки на сервере, но они сохранены локально');
      router.back();
    } finally {
      setSaving(false);
    }
  }, [settings, saving, router]);

  const validateTimeFormat = (time: string): boolean => {
    const timeRegex = /^([01]?[0-9]|2[0-3]):([0-5][0-9])$/;
    return timeRegex.test(time);
  };

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
      <View style={[styles.header, { paddingTop: insets.top }]}>
        <TouchableOpacity onPress={() => router.back()}>
          <MaterialIcons name="arrow-back" size={24} color={textColor} />
        </TouchableOpacity>
        <ThemedText style={[styles.headerTitle, { color: textColor }]}>
          Уведомления
        </ThemedText>
        <View style={{ width: 24 }} />
      </View>

      <ScrollView style={styles.content} showsVerticalScrollIndicator={false}>
        {/* Push Notifications */}
        <View style={[styles.section, { backgroundColor: surfaceColor, borderColor }]}>
          <View style={styles.sectionHeader}>
            <MaterialIcons name="notifications" size={24} color={primaryColor} />
            <ThemedText style={[styles.sectionTitle, { color: textColor }]}>
              Push-уведомления
            </ThemedText>
          </View>

          <View style={styles.settingRow}>
            <View style={styles.settingInfo}>
              <ThemedText style={[styles.settingLabel, { color: textColor }]}>
                Включить push-уведомления
              </ThemedText>
              <ThemedText style={[styles.settingDescription, { color: textColor + '80' }]}>
                Получать уведомления о новых сообщениях
              </ThemedText>
            </View>
            <Switch
              value={settings.push_enabled}
              onValueChange={(value) => setSettings({ ...settings, push_enabled: value })}
              trackColor={{ false: '#767577', true: primaryColor }}
            />
          </View>
        </View>

        {/* Web Push Notifications (only on web) */}
        {Platform.OS === 'web' && webPushSupported && (
          <View style={[styles.section, { backgroundColor: surfaceColor, borderColor }]}>
            <View style={styles.sectionHeader}>
              <MaterialIcons name="language" size={24} color={primaryColor} />
              <ThemedText style={[styles.sectionTitle, { color: textColor }]}>
                Веб-уведомления
              </ThemedText>
            </View>

            <View style={styles.settingRow}>
              <View style={styles.settingInfo}>
                <ThemedText style={[styles.settingLabel, { color: textColor }]}>
                  Включить веб-уведомления
                </ThemedText>
                <ThemedText style={[styles.settingDescription, { color: textColor + '80' }]}>
                  Получать уведомления в браузере
                </ThemedText>
              </View>
              <Switch
                value={webPushEnabled}
                onValueChange={handleWebPushToggle}
                trackColor={{ false: '#767577', true: primaryColor }}
              />
            </View>
          </View>
        )}

        {/* Email Notifications */}
        <View style={[styles.section, { backgroundColor: surfaceColor, borderColor }]}>
          <View style={styles.sectionHeader}>
            <MaterialIcons name="email" size={24} color={primaryColor} />
            <ThemedText style={[styles.sectionTitle, { color: textColor }]}>
              Email-уведомления
            </ThemedText>
          </View>
          
          <View style={styles.settingRow}>
            <View style={styles.settingInfo}>
              <ThemedText style={[styles.settingLabel, { color: textColor }]}>
                Включить email-уведомления
              </ThemedText>
              <ThemedText style={[styles.settingDescription, { color: textColor + '80' }]}>
                Получать уведомления на email
              </ThemedText>
            </View>
            <Switch
              value={settings.email_enabled}
              onValueChange={(value) => setSettings({ ...settings, email_enabled: value })}
              trackColor={{ false: '#767577', true: primaryColor }}
            />
          </View>

          {settings.email_enabled && (
            <View style={styles.textInputRow}>
              <ThemedText style={[styles.settingLabel, { color: textColor }]}>
                Email для уведомлений
              </ThemedText>
              <TextInput
                style={[styles.textInput, { color: textColor, borderColor }]}
                value={settings.email}
                onChangeText={(text) => setSettings({ ...settings, email: text })}
                placeholder="example@email.com"
                placeholderTextColor={textColor + '60'}
                keyboardType="email-address"
                autoCapitalize="none"
              />
            </View>
          )}
        </View>

        {/* Quiet Hours */}
        <View style={[styles.section, { backgroundColor: surfaceColor, borderColor }]}>
          <View style={styles.sectionHeader}>
            <MaterialIcons name="bedtime" size={24} color={primaryColor} />
            <ThemedText style={[styles.sectionTitle, { color: textColor }]}>
              Тихие часы
            </ThemedText>
          </View>

          <View style={styles.settingRow}>
            <View style={styles.settingInfo}>
              <ThemedText style={[styles.settingLabel, { color: textColor }]}>
                Включить тихие часы
              </ThemedText>
              <ThemedText style={[styles.settingDescription, { color: textColor + '80' }]}>
                Не беспокоить в указанное время
              </ThemedText>
            </View>
            <Switch
              value={settings.quiet_hours_enabled}
              onValueChange={(value) => {
                console.log('[NotificationSettings] Quiet hours toggled:', value);
                setSettings({ ...settings, quiet_hours_enabled: value });
              }}
              trackColor={{ false: '#767577', true: primaryColor }}
            />
          </View>

          {settings.quiet_hours_enabled && (
            <View style={styles.timeInputs}>
              <View style={styles.timeInputRow}>
                <ThemedText style={[styles.settingLabel, { color: textColor }]}>
                  С
                </ThemedText>
                <TextInput
                  style={[styles.timeInput, { color: textColor, borderColor }]}
                  value={settings.quiet_hours_start}
                  onChangeText={(text) => setSettings({ ...settings, quiet_hours_start: text })}
                  placeholder="22:00"
                  placeholderTextColor={textColor + '60'}
                  keyboardType="numbers-and-punctuation"
                  maxLength={5}
                />
              </View>

              <View style={styles.timeInputRow}>
                <ThemedText style={[styles.settingLabel, { color: textColor }]}>
                  До
                </ThemedText>
                <TextInput
                  style={[styles.timeInput, { color: textColor, borderColor }]}
                  value={settings.quiet_hours_end}
                  onChangeText={(text) => setSettings({ ...settings, quiet_hours_end: text })}
                  placeholder="09:00"
                  placeholderTextColor={textColor + '60'}
                  keyboardType="numbers-and-punctuation"
                  maxLength={5}
                />
              </View>
            </View>
          )}
        </View>

        {/* Hybrid Mode */}
        <View style={[styles.section, { backgroundColor: surfaceColor, borderColor }]}>
          <View style={styles.sectionHeader}>
            <MaterialIcons name="sync-alt" size={24} color={primaryColor} />
            <ThemedText style={[styles.sectionTitle, { color: textColor }]}>
              Гибридный режим (XMPP)
            </ThemedText>
          </View>

          <View style={styles.settingRow}>
            <View style={styles.settingInfo}>
              <ThemedText style={[styles.settingLabel, { color: textColor }]}>
                Включить гибридный режим
              </ThemedText>
              <ThemedText style={[styles.settingDescription, { color: textColor + '80' }]}>
                Синхронизация между WebSocket и XMPP
              </ThemedText>
            </View>
            <Switch
              value={settings.hybrid_mode_enabled}
              onValueChange={(value) => setSettings({ ...settings, hybrid_mode_enabled: value })}
              trackColor={{ false: '#767577', true: primaryColor }}
            />
          </View>
        </View>

        <TouchableOpacity
          style={[styles.saveButton, { backgroundColor: primaryColor }]}
          onPress={saveSettings}
          disabled={saving}
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
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '600',
    marginLeft: 12,
  },
  settingRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: 16,
  },
  settingInfo: {
    flex: 1,
  },
  settingLabel: {
    fontSize: 16,
    fontWeight: '500',
    marginBottom: 4,
  },
  settingDescription: {
    fontSize: 14,
  },
  textInputRow: {
    marginTop: 12,
  },
  textInput: {
    borderWidth: 1,
    borderRadius: 8,
    padding: 12,
    fontSize: 16,
    marginTop: 8,
  },
  timeInputs: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    marginTop: 12,
  },
  timeInputRow: {
    flex: 1,
    marginRight: 12,
  },
  timeInput: {
    borderWidth: 1,
    borderRadius: 8,
    padding: 12,
    fontSize: 16,
    marginTop: 8,
    textAlign: 'center',
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

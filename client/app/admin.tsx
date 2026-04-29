import React, { useState, useEffect } from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TouchableOpacity,
  ActivityIndicator,
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

interface AuditLog {
  id: string;
  user_id: string;
  action: string;
  details: string;
  created_at: string;
}

interface AdminStats {
  totalUsers: number;
  totalChats: number;
  totalMessages: number;
}

export default function AdminScreen() {
  const router = useRouter();
  const { user, token } = useAuth();
  const insets = useSafeAreaInsets();

  const [loading, setLoading] = useState(true);
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [allUsers, setAllUsers] = useState<any[]>([]);
  const [selectedTab, setSelectedTab] = useState<'logs' | 'users' | 'stats'>('logs');

  const backgroundColor = useThemeColor({}, 'background');
  const surfaceColor = useThemeColor({}, 'surface');
  const textColor = useThemeColor({}, 'text');
  const primaryColor = useThemeColor({}, 'primary');
  const borderColor = useThemeColor({}, 'border');

  useEffect(() => {
    if (user?.role !== 'admin') {
      Alert.alert('Ошибка', 'Доступ запрещен');
      router.back();
      return;
    }
    loadData();
  }, []);

  const loadData = async () => {
    setLoading(true);
    try {
      const [logsRes, usersRes] = await Promise.all([
        api.getAllAuditLogs(),
        api.getAllUsers(),
      ]);

      if (logsRes.data) {
        setAuditLogs(logsRes.data);
      }
      if (usersRes.data) {
        setAllUsers(usersRes.data);
      }
    } catch (error) {
      console.error('Failed to load admin data:', error);
      Alert.alert('Ошибка', 'Не удалось загрузить данные');
    } finally {
      setLoading(false);
    }
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
      <View style={[styles.header, { paddingBottom: insets.bottom }]}>
        <TouchableOpacity onPress={() => router.back()}>
          <MaterialIcons name="arrow-back" size={24} color={textColor} />
        </TouchableOpacity>
        <ThemedText style={[styles.headerTitle, { color: textColor }]}>
          Админ-панель
        </ThemedText>
        <View style={{ width: 24 }} />
      </View>

      {/* Tab Selector */}
      <View style={[styles.tabContainer, { backgroundColor: surfaceColor, borderColor }]}>
        <TouchableOpacity
          style={[
            styles.tab,
            selectedTab === 'logs' && { backgroundColor: primaryColor },
          ]}
          onPress={() => setSelectedTab('logs')}
        >
          <ThemedText
            style={[
              styles.tabText,
              selectedTab === 'logs' && { color: '#fff' },
              { color: textColor },
            ]}
          >
            Логи
          </ThemedText>
        </TouchableOpacity>
        <TouchableOpacity
          style={[
            styles.tab,
            selectedTab === 'users' && { backgroundColor: primaryColor },
          ]}
          onPress={() => setSelectedTab('users')}
        >
          <ThemedText
            style={[
              styles.tabText,
              selectedTab === 'users' && { color: '#fff' },
              { color: textColor },
            ]}
          >
            Пользователи
          </ThemedText>
        </TouchableOpacity>
        <TouchableOpacity
          style={[
            styles.tab,
            selectedTab === 'stats' && { backgroundColor: primaryColor },
          ]}
          onPress={() => setSelectedTab('stats')}
        >
          <ThemedText
            style={[
              styles.tabText,
              selectedTab === 'stats' && { color: '#fff' },
              { color: textColor },
            ]}
          >
            Статистика
          </ThemedText>
        </TouchableOpacity>
      </View>

      <ScrollView style={styles.content}>
        {selectedTab === 'logs' && (
          <View style={styles.section}>
            <ThemedText style={[styles.sectionTitle, { color: textColor }]}>
              Audit Логи
            </ThemedText>
            {auditLogs.length === 0 ? (
              <ThemedText style={[styles.emptyText, { color: textColor + '80' }]}>
                Нет записей
              </ThemedText>
            ) : (
              auditLogs.map((log) => (
                <View key={log.id} style={[styles.logItem, { backgroundColor: surfaceColor, borderColor }]}>
                  <View style={styles.logHeader}>
                    <ThemedText style={[styles.logAction, { color: primaryColor }]}>
                      {log.action}
                    </ThemedText>
                    <ThemedText style={[styles.logDate, { color: textColor + '60' }]}>
                      {new Date(log.created_at).toLocaleString('ru-RU')}
                    </ThemedText>
                  </View>
                  <ThemedText style={[styles.logDetails, { color: textColor }]}>
                    {log.details}
                  </ThemedText>
                  <ThemedText style={[styles.logUserId, { color: textColor + '60' }]}>
                    User ID: {log.user_id}
                  </ThemedText>
                </View>
              ))
            )}
          </View>
        )}

        {selectedTab === 'users' && (
          <View style={styles.section}>
            <ThemedText style={[styles.sectionTitle, { color: textColor }]}>
              Пользователи ({allUsers.length})
            </ThemedText>
            {allUsers.length === 0 ? (
              <ThemedText style={[styles.emptyText, { color: textColor + '80' }]}>
                Нет пользователей
              </ThemedText>
            ) : (
              allUsers.map((u) => (
                <View key={u.id} style={[styles.userItem, { backgroundColor: surfaceColor, borderColor }]}>
                  <View style={styles.userHeader}>
                    <ThemedText style={[styles.userName, { color: textColor }]}>
                      {u.first_name} {u.last_name}
                    </ThemedText>
                    <View style={[
                      styles.roleBadge,
                      { backgroundColor: u.role === 'admin' ? '#4CAF50' : textColor + '20' }
                    ]}>
                      <ThemedText style={[
                        styles.roleText,
                        { color: u.role === 'admin' ? '#fff' : textColor }
                      ]}>
                        {u.role || 'user'}
                      </ThemedText>
                    </View>
                  </View>
                  <ThemedText style={[styles.userEmail, { color: textColor + '60' }]}>
                    {u.email || 'Нет email'}
                  </ThemedText>
                  <ThemedText style={[styles.userId, { color: textColor + '40' }]}>
                    ID: {u.id}
                  </ThemedText>
                </View>
              ))
            )}
          </View>
        )}

        {selectedTab === 'stats' && (
          <View style={styles.section}>
            <ThemedText style={[styles.sectionTitle, { color: textColor }]}>
              Статистика
            </ThemedText>
            <View style={[styles.statCard, { backgroundColor: surfaceColor, borderColor }]}>
              <MaterialIcons name="people" size={32} color={primaryColor} />
              <View>
                <ThemedText style={[styles.statValue, { color: textColor }]}>
                  {allUsers.length}
                </ThemedText>
                <ThemedText style={[styles.statLabel, { color: textColor + '60' }]}>
                  Пользователей
                </ThemedText>
              </View>
            </View>
            <View style={[styles.statCard, { backgroundColor: surfaceColor, borderColor }]}>
              <MaterialIcons name="chat" size={32} color={primaryColor} />
              <View>
                <ThemedText style={[styles.statValue, { color: textColor }]}>
                  {auditLogs.length}
                </ThemedText>
                <ThemedText style={[styles.statLabel, { color: textColor + '60' }]}>
                  Audit записей
                </ThemedText>
              </View>
            </View>
          </View>
        )}
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
  tabContainer: {
    flexDirection: 'row',
    margin: 16,
    borderRadius: 8,
    borderWidth: 1,
    padding: 4,
  },
  tab: {
    flex: 1,
    paddingVertical: 8,
    alignItems: 'center',
    borderRadius: 6,
  },
  tabText: {
    fontSize: 14,
    fontWeight: '600',
  },
  content: {
    flex: 1,
    padding: 16,
  },
  section: {
    paddingBottom: 16,
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '600',
    marginBottom: 16,
  },
  emptyText: {
    fontSize: 14,
    textAlign: 'center',
    padding: 20,
  },
  logItem: {
    borderRadius: 8,
    padding: 12,
    marginBottom: 8,
    borderWidth: 1,
  },
  logHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 8,
  },
  logAction: {
    fontSize: 14,
    fontWeight: '600',
  },
  logDate: {
    fontSize: 12,
  },
  logDetails: {
    fontSize: 14,
    marginBottom: 4,
  },
  logUserId: {
    fontSize: 12,
  },
  userItem: {
    borderRadius: 8,
    padding: 12,
    marginBottom: 8,
    borderWidth: 1,
  },
  userHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 8,
  },
  userName: {
    fontSize: 16,
    fontWeight: '600',
  },
  roleBadge: {
    paddingHorizontal: 8,
    paddingVertical: 4,
    borderRadius: 12,
  },
  roleText: {
    fontSize: 12,
    fontWeight: '600',
  },
  userEmail: {
    fontSize: 14,
    marginBottom: 4,
  },
  userId: {
    fontSize: 12,
  },
  statCard: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 16,
    borderRadius: 12,
    padding: 16,
    marginBottom: 12,
    borderWidth: 1,
  },
  statValue: {
    fontSize: 24,
    fontWeight: '700',
  },
  statLabel: {
    fontSize: 14,
  },
});

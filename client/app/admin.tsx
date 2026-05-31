import React, { useState, useEffect, useCallback } from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TouchableOpacity,
  ActivityIndicator,
  Alert,
  Platform,
  Modal,
  TextInput,
} from 'react-native';
import { useRouter } from 'expo-router';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { useFocusEffect } from '@react-navigation/native';
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
  const { user, token, isLoading: isAuthLoading, refreshUser } = useAuth();
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

  // Check admin role on focus and refresh user data from server
  useFocusEffect(
    useCallback(() => {
      if (isAuthLoading) return;
      if (!user) {
        router.replace('/login');
        return;
      }
      // Always refresh user data from server to catch role changes
      refreshUser().then(() => {
        // Re-check role after refresh (user state will update on next render)
        // For immediate check, fetch current user from API
        api.getCurrentUser().then(res => {
          if (res.data && res.data.role !== 'admin') {
            Alert.alert('Доступ запрещен', 'Ваша роль была изменена');
            router.replace('/');
          } else if (res.data && res.data.role === 'admin') {
            loadData();
          }
        });
      });
    }, [isAuthLoading])
  );

  const loadData = async () => {
    setLoading(true);
    try {
      const [logsRes, usersRes] = await Promise.all([
        api.getAllAuditLogs(),
        api.getAllUsers(),
      ]);

      if (logsRes.data) {
        setAuditLogs(Array.isArray(logsRes.data) ? logsRes.data : (logsRes.data as any).logs || (logsRes.data as any).audit_logs || []);
      }
      if (usersRes.data) {
        setAllUsers(Array.isArray(usersRes.data) ? usersRes.data : (usersRes.data as any).users || []);
      }
    } catch (error) {
      console.error('Failed to load admin data:', error);
      Alert.alert('Ошибка', 'Не удалось загрузить данные');
    } finally {
      setLoading(false);
    }
  };

  const [resetPasswordModalVisible, setResetPasswordModalVisible] = useState(false);
  const [resetPasswordUser, setResetPasswordUser] = useState<any>(null);
  const [newPassword, setNewPassword] = useState('');

  const handleDeleteLogs = async () => {
    Alert.alert(
      'Подтверждение',
      'Вы уверены, что хотите удалить все аудит логи?',
      [
        { text: 'Отмена', style: 'cancel' },
        {
          text: 'Удалить',
          style: 'destructive',
          onPress: async () => {
            try {
              const res = await api.deleteAuditLogs();
              if (res.data) {
                Alert.alert('Успешно', `Удалено ${res.data.deleted_count} записей`);
                setAuditLogs([]);
              }
            } catch (error) {
              Alert.alert('Ошибка', 'Не удалось удалить логи');
            }
          },
        },
      ]
    );
  };

  const handleOpenResetPassword = (user: any) => {
    setResetPasswordUser(user);
    setNewPassword('');
    setResetPasswordModalVisible(true);
  };

  const handleResetPassword = async () => {
    if (!newPassword || newPassword.length < 8) {
      Alert.alert('Ошибка', 'Пароль должен быть минимум 8 символов');
      return;
    }
    // Check for password requirements (uppercase, lowercase, digit, special)
    const hasUpper = /[A-Z]/.test(newPassword);
    const hasLower = /[a-z]/.test(newPassword);
    const hasDigit = /[0-9]/.test(newPassword);
    const hasSpecial = /[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]/.test(newPassword);
    
    if (!hasUpper || !hasLower || !hasDigit || !hasSpecial) {
      Alert.alert('Ошибка', 'Пароль должен содержать: заглавную букву, строчную букву, цифру и спецсимвол');
      return;
    }
    try {
      const res = await api.adminResetPassword(resetPasswordUser.id, newPassword);
      if (res.data) {
        Alert.alert('Успешно', 'Пароль сброшен. Пользователь должен войти заново.');
        setResetPasswordModalVisible(false);
        setResetPasswordUser(null);
        setNewPassword('');
      } else {
        Alert.alert('Ошибка', res.error?.message || 'Не удалось сбросить пароль');
      }
    } catch (e: any) {
      Alert.alert('Ошибка', e.message || 'Не удалось сбросить пароль');
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
            <View style={styles.sectionHeader}>
              <ThemedText style={[styles.sectionTitle, { color: textColor }]}>
                Audit Логи
              </ThemedText>
              {auditLogs.length > 0 && (
                <TouchableOpacity
                  style={[styles.deleteButton, { backgroundColor: '#dc3545' }]}
                  onPress={handleDeleteLogs}
                >
                  <MaterialIcons name="delete" size={18} color="#fff" />
                  <ThemedText style={styles.deleteButtonText}>Очистить все</ThemedText>
                </TouchableOpacity>
              )}
            </View>
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
                  {u.id !== user?.id && (
                    <View style={styles.userActions}>
                      <TouchableOpacity
                        style={[styles.actionBtn, { backgroundColor: '#2196F3' }]}
                        onPress={() => handleOpenResetPassword(u)}>
                        <MaterialIcons name="lock-reset" size={18} color="#fff" />
                        <Text style={styles.actionBtnText}>Сброс пароля</Text>
                      </TouchableOpacity>
                      <TouchableOpacity
                        style={[styles.actionBtn, { backgroundColor: u.role === 'admin' ? '#FF9800' : '#4CAF50' }]}
                        onPress={async () => {
                          const newRole = u.role === 'admin' ? 'user' : 'admin';
                          const doChange = async () => {
                            try {
                              const res = await api.adminChangeRole(u.id, newRole);
                              if (res.data) {
                                setAllUsers(prev => prev.map(usr => usr.id === u.id ? { ...usr, role: newRole } : usr));
                              } else {
                                Alert.alert('Ошибка', res.error?.message || 'Не удалось сменить роль');
                              }
                            } catch (e: any) {
                              Alert.alert('Ошибка', e.message || 'Не удалось сменить роль');
                            }
                          };
                          if (Platform.OS === 'web') {
                            if (window.confirm(`${newRole === 'admin' ? 'Назначить' : 'Снять'} администратора для ${u.first_name}?`)) doChange();
                          } else {
                            Alert.alert('Смена роли', `${newRole === 'admin' ? 'Назначить' : 'Снять'} администратора для ${u.first_name}?`, [
                              { text: 'Отмена', style: 'cancel' },
                              { text: 'ОК', onPress: doChange },
                            ]);
                          }
                        }}>
                        <MaterialIcons name={u.role === 'admin' ? 'remove-moderator' : 'admin-panel-settings'} size={18} color="#fff" />
                        <Text style={styles.actionBtnText}>{u.role === 'admin' ? 'Снять админа' : 'Дать админа'}</Text>
                      </TouchableOpacity>
                      <TouchableOpacity
                        style={[styles.actionBtn, { backgroundColor: '#f44336' }]}
                        onPress={async () => {
                          const doDelete = async () => {
                            try {
                              const res = await api.adminDeleteUser(u.id);
                              if (res.data || !res.error) {
                                setAllUsers(prev => prev.filter(usr => usr.id !== u.id));
                              } else {
                                Alert.alert('Ошибка', res.error?.message || 'Не удалось удалить');
                              }
                            } catch (e: any) {
                              Alert.alert('Ошибка', e.message || 'Не удалось удалить');
                            }
                          };
                          if (Platform.OS === 'web') {
                            if (window.confirm(`Удалить пользователя ${u.first_name} ${u.last_name}? Это действие необратимо!`)) doDelete();
                          } else {
                            Alert.alert('Удаление', `Удалить пользователя ${u.first_name} ${u.last_name}? Это действие необратимо!`, [
                              { text: 'Отмена', style: 'cancel' },
                              { text: 'Удалить', style: 'destructive', onPress: doDelete },
                            ]);
                          }
                        }}>
                        <MaterialIcons name="delete" size={18} color="#fff" />
                        <Text style={styles.actionBtnText}>Удалить</Text>
                      </TouchableOpacity>
                    </View>
                  )}
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

      <Modal
        visible={resetPasswordModalVisible}
        transparent={true}
        animationType="slide"
        onRequestClose={() => setResetPasswordModalVisible(false)}>
        <View style={styles.modalOverlay}>
          <View style={[styles.modalContent, { backgroundColor: surfaceColor }]}>
            <ThemedText style={[styles.modalTitle, { color: textColor }]}>
              Сброс пароля
            </ThemedText>
            <ThemedText style={[styles.modalText, { color: textColor + '80' }]}>
              {resetPasswordUser?.first_name} {resetPasswordUser?.last_name}
            </ThemedText>
            <TextInput
              style={[styles.modalInput, { backgroundColor: backgroundColor, color: textColor, borderColor }]}
              placeholder="Заглавная, строчная, цифра, спецсимвол (мин. 8 символов)"
              placeholderTextColor={textColor + '60'}
              value={newPassword}
              onChangeText={setNewPassword}
              secureTextEntry
            />
            <View style={styles.modalButtons}>
              <TouchableOpacity
                style={[styles.modalButton, { backgroundColor: '#999' }]}
                onPress={() => setResetPasswordModalVisible(false)}>
                <Text style={styles.modalButtonText}>Отмена</Text>
              </TouchableOpacity>
              <TouchableOpacity
                style={[styles.modalButton, { backgroundColor: '#2196F3' }]}
                onPress={handleResetPassword}>
                <Text style={styles.modalButtonText}>Сбросить</Text>
              </TouchableOpacity>
            </View>
          </View>
        </View>
      </Modal>
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
  sectionHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 16,
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '600',
  },
  deleteButton: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 12,
    paddingVertical: 8,
    borderRadius: 6,
    gap: 6,
  },
  deleteButtonText: {
    color: '#fff',
    fontSize: 14,
    fontWeight: '600',
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
  userActions: {
    flexDirection: 'row',
    gap: 8,
    marginTop: 8,
  },
  actionBtn: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 12,
    paddingVertical: 6,
    borderRadius: 6,
    gap: 4,
  },
  actionBtnText: {
    color: '#fff',
    fontSize: 12,
    fontWeight: '600',
  },
  modalOverlay: {
    flex: 1,
    backgroundColor: 'rgba(0, 0, 0, 0.5)',
    justifyContent: 'center',
    alignItems: 'center',
  },
  modalContent: {
    width: '80%',
    maxWidth: 400,
    padding: 20,
    borderRadius: 12,
  },
  modalTitle: {
    fontSize: 18,
    fontWeight: '600',
    marginBottom: 8,
  },
  modalText: {
    fontSize: 14,
    marginBottom: 16,
  },
  modalInput: {
    borderWidth: 1,
    borderRadius: 8,
    padding: 12,
    marginBottom: 16,
    fontSize: 16,
  },
  modalButtons: {
    flexDirection: 'row',
    gap: 12,
  },
  modalButton: {
    flex: 1,
    paddingVertical: 12,
    borderRadius: 8,
    alignItems: 'center',
  },
  modalButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
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

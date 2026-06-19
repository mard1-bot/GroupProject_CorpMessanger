import { useEffect, useState } from 'react';
import { View, FlatList, TouchableOpacity, StyleSheet, Image, ActivityIndicator, Alert, Platform } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useNavigation } from '@react-navigation/native';
import { MaterialIcons } from '@expo/vector-icons';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { useThemeColor } from '@/hooks/use-theme-color';
import { useAuth } from '@/contexts/auth-context';
import { api, ChatMember } from '@/services/api';
import md5 from 'md5';

import { getUserAvatarUrl } from '@/utils/avatar';

export default function ChatMembersScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const router = useRouter();
  const navigation = useNavigation();
  const { user } = useAuth();

  const [members, setMembers] = useState<ChatMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [removing, setRemoving] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [chatType, setChatType] = useState<string>('');
  const [currentUserRole, setCurrentUserRole] = useState<string>('');
  const [availableUsers, setAvailableUsers] = useState<Array<{id: string, email: string, first_name: string, last_name: string}>>([]);

  const primaryColor = useThemeColor({}, 'primary');
  const textColor = useThemeColor({}, 'text');
  const surfaceColor = useThemeColor({}, 'surface');
  const iconColor = useThemeColor({}, 'icon');

  useEffect(() => {
    navigation.setOptions({
      title: 'Участники',
      headerShown: true,
      headerLeft: () => (
        <TouchableOpacity onPress={() => router.back()} style={styles.backButton}>
          <MaterialIcons name="arrow-back" size={24} color={primaryColor} />
        </TouchableOpacity>
      ),
    });
  }, [navigation, router, primaryColor]);

  useEffect(() => {
    if (!id) return;
    loadMembers();
  }, [id]);

  const loadMembers = async () => {
    try {
      const res = await api.getChatById(id!);
      console.log('loadMembers response:', res.data);
      if (res.data) {
        setMembers(res.data.members || []);
        setChatType(res.data.type || '');
        // Find current user's role
        const currentMember = res.data.members?.find(m => m.user_id === user?.id);
        console.log('Current member found:', currentMember);
        if (currentMember) {
          setCurrentUserRole(currentMember.role);
          console.log('Set currentUserRole to:', currentMember.role);
        }
        console.log('Chat type:', res.data.type);
      }
    } catch (error) {
      console.error('Failed to load members:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleRemoveMember = async (memberId: string, memberName: string) => {
    const doRemove = async () => {
      setRemoving(memberId);
      try {
        const res = await api.removeChatMember(id!, memberId);
        if (res.data) {
          setMembers(prev => prev.filter(m => m.user_id !== memberId));
          if (Platform.OS === 'web') {
            window.alert('Участник удален из чата');
          }
        } else if (res.error) {
          Alert.alert('Ошибка', res.error.message);
        }
      } catch (error) {
        Alert.alert('Ошибка', 'Не удалось удалить участника');
      } finally {
        setRemoving(null);
      }
    };

    if (Platform.OS === 'web') {
      const confirmed = window.confirm(`Удалить ${memberName} из чата?`);
      if (confirmed) doRemove();
    } else {
      Alert.alert(
        'Удалить участника',
        `Удалить ${memberName} из чата?`,
        [
          { text: 'Отмена', style: 'cancel' },
          { text: 'Удалить', style: 'destructive', onPress: doRemove }
        ]
      );
    }
  };

  const handleLeaveChat = () => {
    const isOwner = currentUserRole === 'owner';
    const isLastMember = members.length === 1;
    
    // Owner can leave only if they are the last member
    if (isOwner && !isLastMember) {
      Alert.alert(
        'Нельзя выйти',
        'Вы являетесь владельцем чата. Перед выходом необходимо передать права владельца другому участнику или удалить всех участников.'
      );
      return;
    }

    const doLeave = async () => {
      try {
        const res = await api.removeChatMember(id!, user!.id);
        if (res.data) {
          // If chat was deleted (last member left), show different message
          if (res.data.chat_deleted) {
            if (Platform.OS === 'web') {
              window.alert('Чат удален, так как вы были последним участником');
            }
          } else {
            if (Platform.OS === 'web') {
              window.alert('Вы вышли из чата');
            }
          }
          router.push('/(tabs)');
        } else if (res.error) {
          Alert.alert('Ошибка', res.error.message);
        }
      } catch (error) {
        Alert.alert('Ошибка', 'Не удалось выйти из чата');
      }
    };
    
    if (Platform.OS === 'web') {
      const confirmed = window.confirm('Вы уверены, что хотите выйти из этого чата?');
      if (confirmed) doLeave();
    } else {
      Alert.alert(
        'Выйти из чата',
        'Вы уверены, что хотите выйти из этого чата?',
        [
          { text: 'Отмена', style: 'cancel' },
          { text: 'Выйти', style: 'destructive', onPress: doLeave }
        ]
      );
    }
  };

  const loadAvailableUsers = async () => {
    try {
      // Get all users except current members
      const memberIds = new Set(members.map(m => m.user_id));
      const res = await api.getUsers('', 50);
      if (res.data) {
        const filtered = res.data.filter(u => !memberIds.has(u.id) && u.id !== user?.id);
        setAvailableUsers(filtered);
      }
    } catch (error) {
      console.error('Failed to load users:', error);
    }
  };

  const handleAddMember = async (userId: string, userName: string) => {
    try {
      const res = await api.addChatMember(id!, userId, 'member');
      if (res.data) {
        if (Platform.OS === 'web') {
          window.alert(`${userName} добавлен в чат`);
        }
        setAdding(false);
        loadMembers();
      } else if (res.error) {
        Alert.alert('Ошибка', res.error.message);
      }
    } catch (error) {
      Alert.alert('Ошибка', 'Не удалось добавить участника');
    }
  };

  const handleMakeAdmin = async (memberId: string, memberName: string) => {
    const doMakeAdmin = async () => {
      try {
        const res = await api.addChatMember(id!, memberId, 'admin');
        if (res.data) {
          if (Platform.OS === 'web') {
            window.alert(`${memberName} назначен администратором`);
          }
          loadMembers();
        } else if (res.error) {
          Alert.alert('Ошибка', res.error.message);
        }
      } catch (error) {
        Alert.alert('Ошибка', 'Не удалось назначить администратора');
      }
    };

    if (Platform.OS === 'web') {
      const confirmed = window.confirm(`Назначить ${memberName} администратором чата?`);
      if (confirmed) doMakeAdmin();
    } else {
      Alert.alert(
        'Назначить администратором',
        `Назначить ${memberName} администратором чата?`,
        [
          { text: 'Отмена', style: 'cancel' },
          { text: 'Назначить', onPress: doMakeAdmin }
        ]
      );
    }
  };

  const canAddMember = currentUserRole === 'owner' || currentUserRole === 'admin';
  const canMakeAdmin = currentUserRole === 'owner';
  const canDemoteAdmin = currentUserRole === 'owner';

  const handleDemoteAdmin = async (memberId: string, memberName: string) => {
    const doDemote = async () => {
      try {
        // Use addChatMember with role 'member' to demote
        const res = await api.addChatMember(id!, memberId, 'member');
        if (res.data) {
          if (Platform.OS === 'web') {
            window.alert(`${memberName} теперь обычный участник`);
          }
          loadMembers();
        } else if (res.error) {
          Alert.alert('Ошибка', res.error.message);
        }
      } catch (error) {
        Alert.alert('Ошибка', 'Не удалось снять администратора');
      }
    };

    if (Platform.OS === 'web') {
      const confirmed = window.confirm(`Снять ${memberName} с должности администратора?`);
      if (confirmed) doDemote();
    } else {
      Alert.alert(
        'Снять администратора',
        `Снять ${memberName} с должности администратора?`,
        [
          { text: 'Отмена', style: 'cancel' },
          { text: 'Снять', style: 'destructive', onPress: doDemote }
        ]
      );
    }
  };

  const canRemoveMember = (targetRole: string, targetId: string) => {
    // Can't remove yourself through this button (use Leave instead)
    if (targetId === user?.id) return false;
    // Owner can remove anyone except themselves
    if (currentUserRole === 'owner') return true;
    // Admin can only remove regular members
    if (currentUserRole === 'admin' && targetRole === 'member') return true;
    return false;
  };

  const renderMember = ({ item }: { item: ChatMember }) => {
    const memberUser = item.user;
    if (!memberUser) return null;
    const isCurrentUser = item.user_id === user?.id;
    const showRemove = canRemoveMember(item.role, item.user_id);
    const showMakeAdmin = canMakeAdmin && item.role === 'member' && !isCurrentUser;
    const showDemote = canDemoteAdmin && item.role === 'admin' && !isCurrentUser;
    
    console.log('Render member:', {
      name: `${memberUser.first_name} ${memberUser.last_name}`,
      role: item.role,
      isCurrentUser,
      currentUserRole,
      showRemove,
      showMakeAdmin,
      userId: user?.id,
      memberId: item.user_id
    });

    return (
      <View style={[styles.memberItem, { backgroundColor: surfaceColor }]}>
        <Image
          source={{ uri: getUserAvatarUrl(memberUser, 48) }}
          style={styles.avatar}
        />
        <View style={styles.memberInfo}>
          <ThemedText style={styles.name}>
            {memberUser.first_name} {memberUser.last_name}
            {isCurrentUser && (
              <ThemedText style={[styles.youLabel, { color: iconColor }]}> (Вы)</ThemedText>
            )}
          </ThemedText>
          <ThemedText style={[styles.username, { color: iconColor }]}>
            @{memberUser.username}
          </ThemedText>
        </View>
        <View style={styles.rightContainer}>
          <View style={[styles.roleBadge, { backgroundColor: item.role === 'owner' ? '#9C27B0' : item.role === 'admin' ? '#4CAF50' : iconColor + '40' }]}>
            <ThemedText style={styles.roleText}>
              {item.role === 'owner' ? 'Владелец' : item.role === 'admin' ? 'Админ' : 'Участник'}
            </ThemedText>
          </View>
          {showMakeAdmin && (
            <TouchableOpacity
              onPress={() => handleMakeAdmin(item.user_id, `${memberUser.first_name} ${memberUser.last_name}`)}
              style={styles.actionButton}>
              <MaterialIcons name="upgrade" size={22} color={primaryColor} />
            </TouchableOpacity>
          )}
          {showDemote && (
            <TouchableOpacity
              onPress={() => handleDemoteAdmin(item.user_id, `${memberUser.first_name} ${memberUser.last_name}`)}
              style={styles.actionButton}>
              <MaterialIcons name="arrow-downward" size={22} color="#FF9800" />
            </TouchableOpacity>
          )}
          {showRemove && (
            <TouchableOpacity
              onPress={() => handleRemoveMember(item.user_id, `${memberUser.first_name} ${memberUser.last_name}`)}
              style={styles.removeButton}
              disabled={removing === item.user_id}>
              {removing === item.user_id ? (
                <ActivityIndicator size="small" color="#FF3B30" />
              ) : (
                <MaterialIcons name="remove-circle" size={24} color="#FF3B30" />
              )}
            </TouchableOpacity>
          )}
        </View>
      </View>
    );
  };

  if (loading) {
    return (
      <ThemedView style={styles.centered}>
        <ThemedText>Загрузка...</ThemedText>
      </ThemedView>
    );
  }

  // Don't show leave button for direct chats
  const showLeaveButton = chatType === 'group';

  return (
    <ThemedView style={styles.container}>
      <FlatList
        data={members}
        keyExtractor={(item) => item.user_id}
        renderItem={renderMember}
        contentContainerStyle={styles.list}
        ListEmptyComponent={
          <ThemedText style={styles.empty}>Нет участников</ThemedText>
        }
        ListHeaderComponent={
          canAddMember ? (
            <View style={styles.headerSection}>
              {!adding ? (
                <TouchableOpacity
                  style={[styles.addButton, { backgroundColor: primaryColor }]}
                  onPress={() => {
                    loadAvailableUsers();
                    setAdding(true);
                  }}>
                  <MaterialIcons name="person-add" size={20} color="#fff" />
                  <ThemedText style={styles.addButtonText}>Добавить участника</ThemedText>
                </TouchableOpacity>
              ) : (
                <View style={[styles.addSection, { backgroundColor: surfaceColor }]}>
                  <View style={styles.addSectionHeader}>
                    <ThemedText style={styles.addSectionTitle}>Выберите пользователя</ThemedText>
                    <TouchableOpacity onPress={() => setAdding(false)}>
                      <MaterialIcons name="close" size={24} color={iconColor} />
                    </TouchableOpacity>
                  </View>
                  {availableUsers.length === 0 ? (
                    <ThemedText style={styles.noUsersText}>Нет доступных пользователей</ThemedText>
                  ) : (
                    availableUsers.map(u => (
                      <TouchableOpacity
                        key={u.id}
                        style={styles.userItem}
                        onPress={() => handleAddMember(u.id, `${u.first_name} ${u.last_name}`)}>
                        <Image
                          source={{ uri: getUserAvatarUrl(u, 40) }}
                          style={styles.userAvatar}
                        />
                        <View style={styles.userInfo}>
                          <ThemedText style={styles.userName}>
                            {u.first_name} {u.last_name}
                          </ThemedText>
                          <ThemedText style={[styles.userEmail, { color: iconColor }]}>
                            {u.email}
                          </ThemedText>
                        </View>
                        <MaterialIcons name="add" size={24} color={primaryColor} />
                      </TouchableOpacity>
                    ))
                  )}
                </View>
              )}
            </View>
          ) : null
        }
        ListFooterComponent={
          showLeaveButton ? (
            <View style={styles.footer}>
              <TouchableOpacity
                style={[styles.leaveButton, { borderColor: '#FF3B30' }]}
                onPress={handleLeaveChat}>
                <MaterialIcons name="exit-to-app" size={20} color="#FF3B30" />
                <ThemedText style={[styles.leaveButtonText, { color: '#FF3B30' }]}>
                  {currentUserRole === 'owner' 
                    ? (members.length === 1 ? 'Выйти и удалить чат' : 'Покинуть чат (требуется передача прав)')
                    : 'Выйти из чата'}
                </ThemedText>
              </TouchableOpacity>
            </View>
          ) : null
        }
      />
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  centered: { flex: 1, justifyContent: 'center', alignItems: 'center' },
  backButton: { paddingHorizontal: 16, paddingVertical: 8 },
  list: { padding: 16 },
  memberItem: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 12,
    borderRadius: 12,
    marginBottom: 8,
  },
  avatar: { width: 48, height: 48, borderRadius: 24 },
  memberInfo: { flex: 1, marginLeft: 12 },
  name: { fontSize: 16, fontWeight: '600' },
  username: { fontSize: 14, marginTop: 2 },
  roleBadge: {
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderRadius: 12,
  },
  roleText: { fontSize: 12, color: '#fff' },
  empty: { textAlign: 'center', marginTop: 40 },
  rightContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  removeButton: {
    padding: 4,
  },
  actionButton: {
    padding: 4,
    marginRight: 4,
  },
  youLabel: {
    fontSize: 14,
    fontWeight: '400',
  },
  footer: {
    marginTop: 24,
    paddingTop: 16,
    borderTopWidth: 1,
    borderTopColor: '#E5E5EA',
  },
  leaveButton: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    paddingVertical: 12,
    paddingHorizontal: 16,
    borderRadius: 8,
    borderWidth: 1,
    gap: 8,
  },
  leaveButtonText: {
    fontSize: 16,
    fontWeight: '500',
  },
  headerSection: {
    marginBottom: 16,
  },
  addButton: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    paddingVertical: 12,
    paddingHorizontal: 16,
    borderRadius: 8,
    gap: 8,
  },
  addButtonText: {
    fontSize: 16,
    fontWeight: '500',
    color: '#fff',
  },
  addSection: {
    borderRadius: 12,
    padding: 12,
  },
  addSectionHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 12,
  },
  addSectionTitle: {
    fontSize: 16,
    fontWeight: '600',
  },
  noUsersText: {
    textAlign: 'center',
    paddingVertical: 20,
    opacity: 0.6,
  },
  userItem: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 8,
    borderBottomWidth: 1,
    borderBottomColor: '#E5E5EA',
  },
  userAvatar: {
    width: 40,
    height: 40,
    borderRadius: 20,
  },
  userInfo: {
    flex: 1,
    marginLeft: 12,
  },
  userName: {
    fontSize: 15,
    fontWeight: '500',
  },
  userEmail: {
    fontSize: 13,
  },
});

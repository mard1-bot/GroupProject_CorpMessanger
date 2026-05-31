import { useState, useCallback, useEffect } from 'react';
import {
  View,
  TextInput,
  FlatList,
  TouchableOpacity,
  StyleSheet,
  ActivityIndicator,
  Platform,
} from 'react-native';
import { useRouter } from 'expo-router';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { useAuth } from '@/contexts/auth-context';
import { api } from '@/services/api';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { useThemeColor } from '@/hooks/use-theme-color';
import { Ionicons } from '@expo/vector-icons';
import { useDebounce } from '@/hooks/use-debounce';

interface User {
  id: string;
  email: string;
  username?: string;
  first_name: string;
  last_name: string;
  avatar?: string;
  status: string;
}

export default function NewChatScreen() {
  const router = useRouter();
  const { token } = useAuth();
  const insets = useSafeAreaInsets();
  const [mode, setMode] = useState<'direct' | 'group'>('direct');
  const [search, setSearch] = useState('');
  const [users, setUsers] = useState<User[]>([]);
  const [selectedUsers, setSelectedUsers] = useState<User[]>([]);
  const [groupTitle, setGroupTitle] = useState('');
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const debouncedSearch = useDebounce(search, 300);

  const backgroundColor = useThemeColor({}, 'background');
  const surfaceColor = useThemeColor({}, 'surface');
  const textColor = useThemeColor({}, 'text');
  const primaryColor = useThemeColor({}, 'primary');
  const borderColor = useThemeColor({}, 'border');

  useEffect(() => {
    if (debouncedSearch.length < 2) {
      setUsers([]);
      return;
    }
    searchUsers(debouncedSearch);
  }, [debouncedSearch]);

  const searchUsers = async (query: string) => {
    // Strip leading @ for username search
    const cleanQuery = query.startsWith('@') ? query.slice(1) : query;
    console.log('Searching for:', cleanQuery);
    setLoading(true);
    try {
      const response = await api.getUsers(cleanQuery, 20);
      console.log('Search response:', response);
      if (response.data) {
        console.log('Found users:', response.data.length, response.data);
        setUsers(response.data);
      } else {
        console.log('No data in response');
        setUsers([]);
      }
    } catch (error) {
      console.error('Search error:', error);
      setUsers([]);
    } finally {
      setLoading(false);
    }
  };

  const toggleUserSelection = (user: User) => {
    if (mode === 'direct') {
      setSelectedUsers([user]);
    } else {
      setSelectedUsers(prev => {
        const exists = prev.find(u => u.id === user.id);
        if (exists) {
          return prev.filter(u => u.id !== user.id);
        }
        return [...prev, user];
      });
    }
  };

  const isSelected = (userId: string) => selectedUsers.some(u => u.id === userId);

  const createChat = async () => {
    if (selectedUsers.length === 0) return;
    
    setCreating(true);
    try {
      const response = await api.createChat({
        type: mode,
        title: mode === 'group' ? groupTitle : undefined,
        member_ids: selectedUsers.map(u => u.id),
      });
      
      if (response.data) {
        router.replace(`/chat/${response.data.id}`);
      } else if (response.error) {
        if (response.error.code === 'chat_exists') {
          alert('Личный чат с этим пользователем уже существует');
        } else {
          alert('Ошибка: ' + response.error.message);
        }
      }
    } catch (error) {
      console.error('Create chat error:', error);
      alert('Не удалось создать чат');
    } finally {
      setCreating(false);
    }
  };

  const getInitials = (user: User) => {
    const fromNames = `${user.first_name?.[0] || ''}${user.last_name?.[0] || ''}`.toUpperCase();
    if (fromNames) return fromNames;
    if (user.email && user.email.length > 0) return user.email[0].toUpperCase();
    return '?';
  };

  const renderUser = useCallback(({ item }: { item: User }) => {
    const selected = isSelected(item.id);
    return (
      <TouchableOpacity
        onPress={() => toggleUserSelection(item)}
        style={[styles.userItem, { backgroundColor: selected ? primaryColor + '20' : surfaceColor }]}>
        <View style={[styles.avatar, { backgroundColor: primaryColor }]}>
          <ThemedText style={styles.avatarText}>{getInitials(item)}</ThemedText>
        </View>
        <View style={styles.userInfo}>
          <ThemedText style={[styles.userName, { color: textColor }]}>
            {item.first_name} {item.last_name}
          </ThemedText>
          <ThemedText style={[styles.userEmail, { color: textColor + '80' }]}>
            {item.username ? '@' + item.username : item.email}
          </ThemedText>
        </View>
        {selected && (
          <Ionicons name="checkmark-circle" size={24} color={primaryColor} />
        )}
      </TouchableOpacity>
    );
  }, [selectedUsers, mode, primaryColor, surfaceColor, textColor]);

  return (
    <ThemedView style={[styles.container, { 
      paddingTop: Platform.OS === 'ios' ? insets.top : insets.top + 10,
      paddingBottom: insets.bottom 
    }]}>
      {/* Mode Toggle */}
      <View style={[styles.modeToggle, { backgroundColor: surfaceColor, borderColor, marginTop: 8 }]}>
        <TouchableOpacity
          onPress={() => { setMode('direct'); setSelectedUsers([]); }}
          style={[styles.modeButton, mode === 'direct' && { backgroundColor: primaryColor }]}>
          <ThemedText style={[styles.modeText, { color: mode === 'direct' ? '#fff' : textColor }]}>
            Личный
          </ThemedText>
        </TouchableOpacity>
        <TouchableOpacity
          onPress={() => { setMode('group'); setSelectedUsers([]); }}
          style={[styles.modeButton, mode === 'group' && { backgroundColor: primaryColor }]}>
          <ThemedText style={[styles.modeText, { color: mode === 'group' ? '#fff' : textColor }]}>
            Группа
          </ThemedText>
        </TouchableOpacity>
      </View>

      {/* Group Title Input */}
      {mode === 'group' && (
        <View style={[styles.titleContainer, { backgroundColor: surfaceColor, borderColor }]}>
          <TextInput
            style={[styles.titleInput, { color: textColor }]}
            placeholder="Название группы..."
            placeholderTextColor={textColor + '60'}
            value={groupTitle}
            onChangeText={setGroupTitle}
          />
        </View>
      )}

      {/* Selected Users Chips */}
      {selectedUsers.length > 0 && (
        <View style={styles.selectedContainer}>
          <FlatList
            horizontal
            data={selectedUsers}
            keyExtractor={(item) => item.id}
            renderItem={({ item }) => (
              <TouchableOpacity
                onPress={() => toggleUserSelection(item)}
                style={[styles.chip, { backgroundColor: primaryColor }]}>
                <ThemedText style={styles.chipText}>
                  {item.first_name} {item.last_name}
                </ThemedText>
                <Ionicons name="close" size={16} color="#fff" />
              </TouchableOpacity>
            )}
            contentContainerStyle={styles.chipList}
          />
        </View>
      )}

      {/* Search */}
      <View style={[styles.searchContainer, { backgroundColor: surfaceColor, borderColor }]}>
        <Ionicons name="search" size={20} color={textColor + '80'} />
        <TextInput
          style={[styles.searchInput, { color: textColor }]}
          placeholder="Поиск по имени или email..."
          placeholderTextColor={textColor + '60'}
          value={search}
          onChangeText={setSearch}
          autoFocus
        />
        {search.length > 0 && (
          <TouchableOpacity onPress={() => setSearch('')}>
            <Ionicons name="close-circle" size={20} color={textColor + '80'} />
          </TouchableOpacity>
        )}
      </View>

      {loading && <ActivityIndicator style={styles.loader} color={primaryColor} />}

      {/* Users List */}
      <FlatList
        data={users}
        keyExtractor={(item) => item.id}
        renderItem={renderUser}
        contentContainerStyle={styles.list}
        ListEmptyComponent={
          !loading && search.length >= 2 ? (
            <ThemedText style={styles.emptyText}>Ничего не найдено</ThemedText>
          ) : null
        }
      />

      {/* Create Button */}
      {selectedUsers.length > 0 && (
        <TouchableOpacity
          onPress={createChat}
          disabled={creating || (mode === 'group' && !groupTitle.trim())}
          style={[styles.createButton, { 
            backgroundColor: (mode === 'group' && !groupTitle.trim()) ? '#ccc' : primaryColor,
            opacity: creating ? 0.7 : 1 
          }]}>
          <ThemedText style={styles.createButtonText}>
            {creating ? 'Создание...' : (mode === 'direct' ? 'Создать чат' : `Создать группу (${selectedUsers.length})`)}
          </ThemedText>
        </TouchableOpacity>
      )}
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  modeToggle: {
    flexDirection: 'row',
    margin: 16,
    marginBottom: 8,
    borderRadius: 10,
    borderWidth: 1,
    padding: 4,
    gap: 4,
  },
  modeButton: {
    flex: 1,
    paddingVertical: 10,
    borderRadius: 8,
    alignItems: 'center',
  },
  modeText: {
    fontSize: 14,
    fontWeight: '600',
  },
  titleContainer: {
    marginHorizontal: 16,
    marginBottom: 8,
    borderRadius: 10,
    borderWidth: 1,
    paddingHorizontal: 12,
    paddingVertical: 10,
  },
  titleInput: {
    fontSize: 16,
  },
  selectedContainer: {
    marginHorizontal: 16,
    marginBottom: 8,
  },
  chipList: {
    gap: 8,
    paddingVertical: 4,
  },
  chip: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 12,
    paddingVertical: 6,
    borderRadius: 16,
    gap: 6,
  },
  chipText: {
    color: '#fff',
    fontSize: 14,
    fontWeight: '500',
  },
  searchContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    margin: 16,
    marginTop: 0,
    paddingHorizontal: 12,
    paddingVertical: 8,
    borderRadius: 10,
    borderWidth: 1,
    gap: 8,
  },
  searchInput: { flex: 1, fontSize: 16, paddingVertical: 4 },
  loader: { marginTop: 20 },
  list: { padding: 16, paddingTop: 0 },
  userItem: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 12,
    borderRadius: 12,
    marginBottom: 8,
    gap: 12,
  },
  avatar: {
    width: 48,
    height: 48,
    borderRadius: 24,
    justifyContent: 'center',
    alignItems: 'center',
  },
  avatarText: { color: '#fff', fontSize: 18, fontWeight: '600' },
  userInfo: { flex: 1 },
  userName: { fontSize: 16, fontWeight: '600' },
  userEmail: { fontSize: 14, marginTop: 2 },
  emptyText: { textAlign: 'center', marginTop: 40, opacity: 0.6 },
  createButton: {
    margin: 16,
    paddingVertical: 14,
    borderRadius: 12,
    alignItems: 'center',
  },
  createButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
  },
});

import { useRouter } from 'expo-router';
import { ScrollView, StyleSheet, TouchableOpacity, View, TextInput, Alert, Platform, Modal, Image, Animated } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { MaterialIcons, Ionicons } from '@expo/vector-icons';
import { useState, useEffect, useRef } from 'react';
import * as ImagePicker from 'expo-image-picker';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Colors } from '@/constants/theme';
import { useColorScheme } from '@/hooks/use-color-scheme';
import { useThemeColor } from '@/hooks/use-theme-color';
import { useAuth } from '@/contexts/auth-context';
import { useThemeStore, type ThemeScheme } from '@/store/theme';
import { api } from '@/services/api';

export default function ProfileScreen() {
  const { user: currentUser, logout, refreshUser } = useAuth();
  const router = useRouter();
  const tintColor = useThemeColor({}, 'tint');
  const colorScheme = useThemeStore((s) => s.colorScheme);
  const setColorScheme = useThemeStore((s) => s.setColorScheme);
  const textColor = useThemeColor({}, 'text');
  const isDark = useColorScheme() === 'dark';
  const selectedOptionTextColor = isDark ? '#fff' : Colors.light.text;
  const bgColor = useThemeColor({}, 'background');
  const surfaceColor = useThemeColor({}, 'surface');
  const borderColor = useThemeColor({}, 'border');

  const [isEditing, setIsEditing] = useState(false);
  const [firstName, setFirstName] = useState(currentUser?.first_name || '');
  const [lastName, setLastName] = useState(currentUser?.last_name || '');
  const [username, setUsername] = useState(currentUser?.username ? `@${currentUser.username}` : '');
  const [avatarMenuVisible, setAvatarMenuVisible] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  
  // Animation for the edit form
  const fadeAnim = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    if (currentUser && !isEditing) {
      setFirstName(currentUser.first_name || '');
      setLastName(currentUser.last_name || '');
      setUsername(currentUser.username ? `@${currentUser.username}` : '');
    }
  }, [currentUser, isEditing]);

  useEffect(() => {
    Animated.timing(fadeAnim, {
      toValue: isEditing ? 1 : 0,
      duration: 300,
      useNativeDriver: true,
    }).start();
  }, [isEditing]);

  const initials = currentUser?.first_name
    ? (currentUser.first_name[0] + (currentUser.last_name?.[0] || '')).toUpperCase()
    : '?';
    
  const getAvatarUrl = (path: string | undefined) => {
    if (!path) return null;
    return path.startsWith('http') ? path : `${process.env.EXPO_PUBLIC_BACKEND_URL || 'http://localhost:8080'}${path}`;
  };

  const handleLogout = async () => {
    await logout();
    router.replace('/(auth)/login');
  };

  const themeOptions: { value: ThemeScheme; label: string; icon: keyof typeof Ionicons.glyphMap }[] = [
    { value: 'light', label: 'Светлая', icon: 'sunny-outline' },
    { value: 'dark', label: 'Тёмная', icon: 'moon-outline' },
    { value: 'system', label: 'Системная', icon: 'settings-outline' },
  ];

  const handleSave = async () => {
    if (!firstName.trim()) {
      Alert.alert('Ошибка', 'Имя не может быть пустым');
      return;
    }
    
    setIsSaving(true);
    const res = await api.updateCurrentUser({
      first_name: firstName.trim(),
      last_name: lastName.trim(),
      username: username.trim()
    });
    
    if (res.error) {
      Alert.alert('Ошибка', res.error.message || 'Не удалось обновить профиль');
      setIsSaving(false);
      return;
    }
    
    Alert.alert('Успех', 'Профиль успешно обновлен!');
    setIsEditing(false);
    setIsSaving(false);
    refreshUser();
  };

  const handlePickImage = async (useCamera: boolean = false) => {
    setAvatarMenuVisible(false);
    try {
      if (Platform.OS === 'web' && useCamera) {
        Alert.alert('Камера', 'Использование камеры доступно только в мобильном приложении');
        return;
      }
      
      const options: ImagePicker.ImagePickerOptions = {
        mediaTypes: ['images'],
        allowsEditing: true,
        aspect: [1, 1],
        quality: 0.8,
      };

      const result = useCamera 
        ? await ImagePicker.launchCameraAsync(options)
        : await ImagePicker.launchImageLibraryAsync(options);

      if (!result.canceled && result.assets && result.assets.length > 0) {
        const asset = result.assets[0];
        const filename = asset.fileName || asset.uri.split('/').pop() || 'avatar.jpg';
        
        let fileBlob: Blob | undefined;
        if (Platform.OS === 'web') {
          const res = await fetch(asset.uri);
          fileBlob = await res.blob();
        }

        setIsSaving(true);
        const uploadRes = await api.uploadAvatar({
          uri: asset.uri,
          name: filename,
          type: asset.mimeType || 'image/jpeg',
          file: fileBlob,
        });

        if (uploadRes.error) {
          Alert.alert('Ошибка', uploadRes.error.message || 'Не удалось загрузить фото');
        } else {
          Alert.alert('Успех', 'Фото профиля обновлено!');
          // Update the user context so the new avatar displays immediately
          await refreshUser();
        }
        setIsSaving(false);
      }
    } catch (error) {
      console.error('Image picker error:', error);
      setIsSaving(false);
    }
  };

  return (
    <SafeAreaView style={[styles.safe, { backgroundColor: bgColor }]} edges={['top']}>
      <ScrollView style={[styles.scroll, { backgroundColor: bgColor }]} showsVerticalScrollIndicator={false}>
        
        {/* Dynamic Header Background */}
        <View style={[styles.headerGradient, { backgroundColor: tintColor, opacity: 0.1 }]} />

        <View style={styles.container}>
          <View style={styles.headerRow}>
            <ThemedText type="title" style={styles.header}>Профиль</ThemedText>
            <TouchableOpacity 
              onPress={() => isEditing ? handleSave() : setIsEditing(true)}
              disabled={isSaving}
              activeOpacity={0.7}
              style={[
                styles.editButton, 
                isEditing ? { backgroundColor: tintColor } : { backgroundColor: surfaceColor, borderWidth: 1, borderColor: borderColor }
              ]}
            >
              <ThemedText style={[styles.editButtonText, isEditing ? { color: '#fff' } : { color: textColor }]}>
                {isSaving ? 'Сохранение...' : isEditing ? 'Сохранить' : 'Изменить'}
              </ThemedText>
            </TouchableOpacity>
          </View>

          {/* Profile Card */}
          <View style={[styles.profileCard, { backgroundColor: surfaceColor, borderColor, shadowColor: isDark ? '#000' : '#888' }]}>
            <TouchableOpacity 
              onPress={() => setAvatarMenuVisible(true)}
              activeOpacity={0.8}
              style={styles.avatarContainer}
            >
              <View style={[styles.avatar, { backgroundColor: tintColor + '30', borderColor: tintColor }]}>
                {currentUser?.avatar ? (
                  <Image source={{ uri: getAvatarUrl(currentUser.avatar) as string }} style={styles.avatarImage} />
                ) : (
                  <ThemedText style={[styles.avatarText, { color: tintColor }]}>{initials}</ThemedText>
                )}
                <View style={[styles.avatarEditBadge, { backgroundColor: tintColor }]}>
                  <MaterialIcons name="edit" size={14} color="#fff" />
                </View>
              </View>
            </TouchableOpacity>
            
            {!isEditing ? (
              <View style={styles.infoContainer}>
                <ThemedText style={styles.name}>{currentUser?.first_name} {currentUser?.last_name}</ThemedText>
                <ThemedText style={[styles.username, { color: tintColor }]}>
                  {currentUser?.username ? `@${currentUser.username}` : currentUser?.email}
                </ThemedText>
                {currentUser?.status && (
                  <View style={[styles.statusBadge, { backgroundColor: tintColor + '15' }]}>
                    <ThemedText style={[styles.statusText, { color: tintColor }]}>{currentUser.status}</ThemedText>
                  </View>
                )}
              </View>
            ) : (
              <Animated.View style={[styles.editForm, { opacity: fadeAnim, transform: [{ translateY: fadeAnim.interpolate({ inputRange: [0, 1], outputRange: [-10, 0] }) }] }]}>
                <View style={styles.inputGroup}>
                  <MaterialIcons name="person" size={20} color={textColor + '80'} style={styles.inputIcon} />
                  <TextInput
                    style={[styles.input, { color: textColor }]}
                    value={firstName}
                    onChangeText={setFirstName}
                    placeholder="Имя"
                    placeholderTextColor={textColor + '50'}
                  />
                </View>
                <View style={styles.inputGroup}>
                  <MaterialIcons name="person-outline" size={20} color={textColor + '80'} style={styles.inputIcon} />
                  <TextInput
                    style={[styles.input, { color: textColor }]}
                    value={lastName}
                    onChangeText={setLastName}
                    placeholder="Фамилия"
                    placeholderTextColor={textColor + '50'}
                  />
                </View>
                <View style={styles.inputGroup}>
                  <MaterialIcons name="alternate-email" size={20} color={textColor + '80'} style={styles.inputIcon} />
                  <TextInput
                    style={[styles.input, { color: textColor }]}
                    value={username}
                    onChangeText={(text) => setUsername(text.startsWith('@') || !text ? text : '@' + text)}
                    placeholder="@username"
                    placeholderTextColor={textColor + '50'}
                    autoCapitalize="none"
                    autoCorrect={false}
                  />
                </View>
              </Animated.View>
            )}
          </View>

          {/* Settings Sections */}
          <View style={styles.settingsSection}>
            <ThemedText type="subtitle" style={styles.sectionTitle}>Внешний вид</ThemedText>
            <View style={styles.themeRow}>
              {themeOptions.map((opt) => {
                const isSelected = colorScheme === opt.value;
                return (
                  <TouchableOpacity
                    key={opt.value}
                    onPress={() => setColorScheme(opt.value)}
                    style={[
                      styles.themeOption,
                      { backgroundColor: isSelected ? tintColor : surfaceColor, borderColor: isSelected ? tintColor : borderColor },
                    ]}
                    activeOpacity={0.8}
                  >
                    <Ionicons name={opt.icon} size={20} color={isSelected ? '#fff' : textColor} style={styles.themeIcon} />
                    <ThemedText style={[styles.themeOptionText, { color: isSelected ? '#fff' : textColor }]}>
                      {opt.label}
                    </ThemedText>
                  </TouchableOpacity>
                );
              })}
            </View>
          </View>

          <View style={styles.settingsSection}>
            <ThemedText type="subtitle" style={styles.sectionTitle}>Настройки</ThemedText>
            
            <View style={[styles.menuCard, { backgroundColor: surfaceColor, borderColor }]}>
              <TouchableOpacity style={styles.menuItem} onPress={() => router.push('/notification-settings')} activeOpacity={0.7}>
                <View style={[styles.menuIconContainer, { backgroundColor: '#FF950020' }]}>
                  <Ionicons name="notifications" size={22} color="#FF9500" />
                </View>
                <ThemedText style={[styles.menuItemText, { color: textColor }]}>Уведомления</ThemedText>
                <MaterialIcons name="chevron-right" size={24} color={textColor + '40'} />
              </TouchableOpacity>
              
              <View style={[styles.menuDivider, { backgroundColor: borderColor }]} />

              <TouchableOpacity style={styles.menuItem} onPress={() => router.push('/messaging-settings')} activeOpacity={0.7}>
                <View style={[styles.menuIconContainer, { backgroundColor: '#34C75920' }]}>
                  <Ionicons name="chatbubbles" size={22} color="#34C759" />
                </View>
                <ThemedText style={[styles.menuItemText, { color: textColor }]}>Чаты и медиа</ThemedText>
                <MaterialIcons name="chevron-right" size={24} color={textColor + '40'} />
              </TouchableOpacity>

              {currentUser?.role === 'admin' && (
                <>
                  <View style={[styles.menuDivider, { backgroundColor: borderColor }]} />
                  <TouchableOpacity style={styles.menuItem} onPress={() => router.push('/admin')} activeOpacity={0.7}>
                    <View style={[styles.menuIconContainer, { backgroundColor: '#FF3B3020' }]}>
                      <MaterialIcons name="admin-panel-settings" size={22} color="#FF3B30" />
                    </View>
                    <ThemedText style={[styles.menuItemText, { color: textColor }]}>Админ-панель</ThemedText>
                    <MaterialIcons name="chevron-right" size={24} color={textColor + '40'} />
                  </TouchableOpacity>
                </>
              )}
            </View>
          </View>

          <TouchableOpacity
            style={[styles.logoutButton, { backgroundColor: '#FF3B3015' }]}
            onPress={handleLogout}
            activeOpacity={0.7}>
            <Ionicons name="log-out-outline" size={20} color="#FF3B30" style={{ marginRight: 8 }} />
            <ThemedText style={[styles.logoutText, { color: '#FF3B30' }]}>Выйти из аккаунта</ThemedText>
          </TouchableOpacity>
          
        </View>
      </ScrollView>

      {/* Avatar Menu Modal */}
      {avatarMenuVisible && (
        <Modal
          visible={true}
          transparent={true}
          animationType="fade"
          onRequestClose={() => setAvatarMenuVisible(false)}>
          <TouchableOpacity
            style={styles.actionMenuOverlay}
            activeOpacity={1}
            onPress={() => setAvatarMenuVisible(false)}>
            <View style={[styles.actionMenuContent, { backgroundColor: surfaceColor }]}>
              <View style={styles.actionMenuHeader}>
                <ThemedText style={styles.actionMenuTitle}>Изменить фото</ThemedText>
              </View>
              <TouchableOpacity style={styles.actionMenuItem} onPress={() => handlePickImage(false)}>
                <Ionicons name="image-outline" size={22} color={textColor} />
                <ThemedText style={styles.actionMenuText}>Выбрать из галереи</ThemedText>
              </TouchableOpacity>
              
              {Platform.OS !== 'web' && (
                <TouchableOpacity style={styles.actionMenuItem} onPress={() => handlePickImage(true)}>
                  <Ionicons name="camera-outline" size={22} color={textColor} />
                  <ThemedText style={styles.actionMenuText}>Сделать фото</ThemedText>
                </TouchableOpacity>
              )}
              <TouchableOpacity style={styles.actionMenuCancel} onPress={() => setAvatarMenuVisible(false)}>
                <ThemedText style={[styles.actionMenuText, { color: '#FF3B30', fontWeight: '600' }]}>Отмена</ThemedText>
              </TouchableOpacity>
            </View>
          </TouchableOpacity>
        </Modal>
      )}
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  scroll: { flex: 1 },
  headerGradient: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    height: 200,
  },
  container: { flex: 1, paddingHorizontal: 20, paddingBottom: 40, paddingTop: 10 },
  headerRow: { 
    flexDirection: 'row', 
    justifyContent: 'space-between', 
    alignItems: 'center', 
    paddingBottom: 20 
  },
  header: { fontSize: 32, fontWeight: '800' },
  editButton: { 
    paddingHorizontal: 16, 
    paddingVertical: 8, 
    borderRadius: 20, 
  },
  editButtonText: { fontSize: 14, fontWeight: '600' },
  
  profileCard: { 
    alignItems: 'center', 
    paddingVertical: 32, 
    paddingHorizontal: 20,
    borderRadius: 24,
    borderWidth: 1,
    shadowOffset: { width: 0, height: 8 },
    shadowOpacity: 0.08,
    shadowRadius: 16,
    elevation: 4,
    marginBottom: 30,
  },
  avatarContainer: {
    marginBottom: 20,
  },
  avatar: {
    width: 110,
    height: 110,
    borderRadius: 55,
    alignItems: 'center',
    justifyContent: 'center',
    overflow: 'hidden',
    borderWidth: 3,
  },
  avatarImage: { width: '100%', height: '100%' },
  avatarText: { fontSize: 36, fontWeight: '700' },
  avatarEditBadge: {
    position: 'absolute',
    bottom: -2,
    right: 0,
    left: 0,
    height: 28,
    alignItems: 'center',
    justifyContent: 'center',
  },
  infoContainer: { alignItems: 'center' },
  name: { fontSize: 24, fontWeight: '700', marginBottom: 6, textAlign: 'center' },
  username: { fontSize: 16, fontWeight: '500', marginBottom: 12 },
  statusBadge: {
    paddingHorizontal: 12,
    paddingVertical: 6,
    borderRadius: 12,
  },
  statusText: { fontSize: 13, fontWeight: '600' },
  
  editForm: { width: '100%' },
  inputGroup: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: 'rgba(150, 150, 150, 0.08)',
    borderRadius: 16,
    marginBottom: 12,
    paddingHorizontal: 16,
    height: 54,
  },
  inputIcon: { marginRight: 12 },
  input: {
    flex: 1,
    fontSize: 16,
    fontWeight: '500',
  },
  
  settingsSection: { marginBottom: 30 },
  sectionTitle: { marginBottom: 16, fontSize: 17, fontWeight: '700', opacity: 0.8, marginLeft: 4 },
  
  themeRow: { flexDirection: 'row', gap: 12 },
  themeOption: {
    flex: 1,
    borderWidth: 1,
    borderRadius: 16,
    paddingVertical: 14,
    alignItems: 'center',
    justifyContent: 'center',
    flexDirection: 'column',
    gap: 8,
  },
  themeIcon: { marginBottom: 4 },
  themeOptionText: { fontSize: 14, fontWeight: '600' },
  
  menuCard: {
    borderRadius: 20,
    borderWidth: 1,
    overflow: 'hidden',
  },
  menuItem: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 16,
  },
  menuIconContainer: {
    width: 36,
    height: 36,
    borderRadius: 10,
    alignItems: 'center',
    justifyContent: 'center',
  },
  menuItemText: {
    fontSize: 16,
    fontWeight: '500',
    marginLeft: 14,
    flex: 1,
  },
  menuDivider: { height: 1, marginLeft: 66 },
  
  logoutButton: { 
    flexDirection: 'row',
    borderRadius: 16, 
    paddingVertical: 16, 
    alignItems: 'center', 
    justifyContent: 'center',
    marginTop: 10,
  },
  logoutText: { fontSize: 16, fontWeight: '700' },
  
  actionMenuOverlay: {
    flex: 1,
    backgroundColor: 'rgba(0,0,0,0.5)',
    justifyContent: 'flex-end',
  },
  actionMenuContent: {
    borderTopLeftRadius: 24,
    borderTopRightRadius: 24,
    paddingBottom: Platform.OS === 'ios' ? 40 : 20,
    overflow: 'hidden',
  },
  actionMenuHeader: {
    alignItems: 'center',
    paddingVertical: 16,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: 'rgba(150,150,150,0.2)',
  },
  actionMenuTitle: {
    fontSize: 16,
    fontWeight: '600',
    opacity: 0.6,
  },
  actionMenuItem: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 16,
    paddingHorizontal: 24,
  },
  actionMenuText: {
    fontSize: 16,
    marginLeft: 16,
    fontWeight: '500',
  },
  actionMenuCancel: {
    marginTop: 8,
    alignItems: 'center',
    paddingVertical: 16,
    backgroundColor: 'rgba(150,150,150,0.1)',
    marginHorizontal: 16,
    borderRadius: 12,
  }
});

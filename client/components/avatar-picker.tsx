import { useState } from 'react';
import { View, TouchableOpacity, StyleSheet, Modal } from 'react-native';
import { MaterialIcons } from '@expo/vector-icons';
import { ThemedText } from './themed-text';
import { ThemedView } from './themed-view';
import { useThemeColor } from '@/hooks/use-theme-color';

const AVATAR_PRESETS = [
  // Generated abstract avatars (using dicebear-like pattern)
  'https://api.dicebear.com/7.x/avataaars/svg?seed=Felix',
  'https://api.dicebear.com/7.x/avataaars/svg?seed=Aneka',
  'https://api.dicebear.com/7.x/avataaars/svg?seed=Zack',
  'https://api.dicebear.com/7.x/avataaars/svg?seed=Molly',
  'https://api.dicebear.com/7.x/avataaars/svg?seed=Bear',
  'https://api.dicebear.com/7.x/avataaars/svg?seed=Luna',
  'https://api.dicebear.com/7.x/avataaars/svg?seed=Max',
  'https://api.dicebear.com/7.x/avataaars/svg?seed=Sasha',
  'https://api.dicebear.com/7.x/avataaars/svg?seed=Rocky',
  'https://api.dicebear.com/7.x/avataaars/svg?seed=Angel',
  'https://api.dicebear.com/7.x/avataaars/svg?seed=Coco',
  'https://api.dicebear.com/7.x/avataaars/svg?seed=Jack',
];

interface AvatarPickerProps {
  visible: boolean;
  onClose: () => void;
  onSelect: (avatarUrl: string) => void;
  currentAvatar?: string;
}

export function AvatarPicker({ visible, onClose, onSelect, currentAvatar }: AvatarPickerProps) {
  const primaryColor = useThemeColor({}, 'primary');
  const surfaceColor = useThemeColor({}, 'surface');
  const textColor = useThemeColor({}, 'text');

  const [selected, setSelected] = useState<string | null>(null);

  const handleSelect = (url: string) => {
    setSelected(url);
    onSelect(url);
    onClose();
  };

  return (
    <Modal
      animationType="slide"
      transparent={true}
      visible={visible}
      onRequestClose={onClose}
    >
      <View style={styles.overlay}>
        <View style={[styles.content, { backgroundColor: surfaceColor }]}>
          <View style={styles.header}>
            <ThemedText style={styles.title}>Выберите аватар</ThemedText>
            <TouchableOpacity onPress={onClose} style={styles.closeButton}>
              <MaterialIcons name="close" size={24} color={textColor} />
            </TouchableOpacity>
          </View>

          <View style={styles.grid}>
            {AVATAR_PRESETS.map((url, index) => (
              <TouchableOpacity
                key={index}
                style={[
                  styles.avatarItem,
                  currentAvatar === url && { borderColor: primaryColor, borderWidth: 3 },
                ]}
                onPress={() => handleSelect(url)}
              >
                {/* Using Material Icon as placeholder since we can't render SVG easily */}
                <MaterialIcons 
                  name="person" 
                  size={40} 
                  color={primaryColor} 
                />
              </TouchableOpacity>
            ))}
          </View>

          <TouchableOpacity
            style={[styles.gravatarButton, { borderColor: primaryColor }]}
            onPress={() => {
              // Use Gravatar based on email - will be handled by parent
              onClose();
            }}
          >
            <ThemedText style={{ color: primaryColor }}>
              Использовать Gravatar
            </ThemedText>
          </TouchableOpacity>
        </View>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  overlay: {
    flex: 1,
    justifyContent: 'flex-end',
    backgroundColor: 'rgba(0,0,0,0.5)',
  },
  content: {
    borderTopLeftRadius: 20,
    borderTopRightRadius: 20,
    padding: 20,
    maxHeight: '80%',
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 20,
  },
  title: {
    fontSize: 18,
    fontWeight: '600',
  },
  closeButton: {
    padding: 4,
  },
  grid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    justifyContent: 'space-between',
    marginBottom: 20,
  },
  avatarItem: {
    width: '23%',
    aspectRatio: 1,
    borderRadius: 12,
    backgroundColor: '#f0f0f0',
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: 10,
    borderWidth: 2,
    borderColor: 'transparent',
  },
  gravatarButton: {
    padding: 16,
    borderRadius: 12,
    borderWidth: 1,
    alignItems: 'center',
  },
});

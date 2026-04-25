import React from 'react';
import {
  View,
  Text,
  TouchableOpacity,
  Modal,
  StyleSheet,
  Alert,
  Platform,
} from 'react-native';
import { MaterialIcons } from '@expo/vector-icons';
import { useThemeColor } from '@/hooks/use-theme-color';
import { api } from '@/services/api';

interface MessageActionMenuProps {
  visible: boolean;
  onClose: () => void;
  messageId: string;
  chatId: string;
  isOwn: boolean;
  isGroup: boolean;
  userRole: string;
  onDelete: (messageId: string) => void;
  onEdit: (messageId: string, content: string) => void;
  onPin: (messageId: string) => void;
  onUnpin: (messageId: string) => void;
  onForward: (messageId: string) => void;
  messageContent?: string;
  isPinned?: boolean;
}

const EMOJI_LIST = ['👍', '❤️', '😂', '😮', '😢', '😡'];

export function MessageActionMenu({
  visible,
  onClose,
  messageId,
  chatId,
  isOwn,
  isGroup,
  userRole,
  onDelete,
  onEdit,
  onPin,
  onUnpin,
  onForward,
  messageContent = '',
  isPinned = false,
}: MessageActionMenuProps) {
  const primaryColor = useThemeColor({}, 'primary');
  const textColor = useThemeColor({}, 'text');
  const surfaceColor = useThemeColor({}, 'surface');
  const iconColor = useThemeColor({}, 'icon');

  const handleAddReaction = async (emoji: string) => {
    console.log('[MessageActionMenu] Adding reaction:', emoji, 'to message:', messageId);
    try {
      await api.addReaction(messageId, emoji);
      console.log('[MessageActionMenu] Reaction added successfully');
      onClose();
    } catch (error) {
      console.error('[MessageActionMenu] Failed to add reaction:', error);
      Alert.alert('Ошибка', 'Не удалось добавить реакцию');
    }
  };

  const handlePin = async () => {
    console.log('[MessageActionMenu] Pinning message:', messageId, 'isPinned:', isPinned);
    try {
      if (isPinned) {
        await api.unpinMessage(chatId, messageId);
        console.log('[MessageActionMenu] Message unpinned successfully');
      } else {
        await api.pinMessage(chatId, messageId);
        console.log('[MessageActionMenu] Message pinned successfully');
      }
      onClose();
    } catch (error) {
      console.error('[MessageActionMenu] Failed to pin/unpin message:', error);
      Alert.alert('Ошибка', 'Не удалось закрепить сообщение');
    }
  };

  const handleForward = async () => {
    console.log('[MessageActionMenu] Forwarding message:', messageId);
    try {
      await api.forwardMessage(chatId, messageId);
      console.log('[MessageActionMenu] Message forwarded successfully');
      onClose();
      Alert.alert('Успешно', 'Сообщение готово для пересылки');
    } catch (error) {
      console.error('[MessageActionMenu] Failed to forward message:', error);
      Alert.alert('Ошибка', 'Не удалось переслать сообщение');
    }
  };

  const canDelete = isOwn || !isGroup || userRole === 'owner' || userRole === 'admin';
  const canEdit = isOwn;
  const canPin = true;

  const actions = [];

  if (canPin) {
    actions.push({
      icon: 'push-pin' as const,
      label: isPinned ? 'Открепить' : 'Закрепить',
      onPress: handlePin,
      color: textColor,
    });
  }

  actions.push({
    icon: 'forward' as const,
    label: 'Переслать',
    onPress: handleForward,
    color: textColor,
  });

  if (canEdit) {
    actions.push({
      icon: 'edit' as const,
      label: 'Редактировать',
      onPress: () => {
        onEdit(messageId, messageContent);
        onClose();
      },
      color: textColor,
    });
  }

  if (canDelete) {
    actions.push({
      icon: 'delete' as const,
      label: 'Удалить',
      onPress: () => {
        onDelete(messageId);
        onClose();
      },
      color: '#dc3545',
    });
  }

  return (
    <Modal
      visible={visible}
      transparent
      animationType="fade"
      onRequestClose={onClose}
    >
      <TouchableOpacity
        style={styles.overlay}
        activeOpacity={1}
        onPress={onClose}
      >
        <View
          style={[styles.container, { backgroundColor: surfaceColor }]}
          onStartShouldSetResponder={() => true}
          onTouchEnd={(e) => e.stopPropagation()}
        >
          {/* Reactions */}
          <View style={styles.reactionsRow}>
            {EMOJI_LIST.map((emoji) => (
              <TouchableOpacity
                key={emoji}
                style={styles.emojiButton}
                onPress={() => handleAddReaction(emoji)}
              >
                <Text style={styles.emoji}>{emoji}</Text>
              </TouchableOpacity>
            ))}
          </View>

          {/* Divider */}
          <View style={[styles.divider, { backgroundColor: iconColor + '20' }]} />

          {/* Actions */}
          <View style={styles.actionsList}>
            {actions.map((action, index) => (
              <TouchableOpacity
                key={index}
                style={[
                  styles.actionRow,
                  index < actions.length - 1 && styles.actionBorder,
                  { borderBottomColor: iconColor + '15' },
                ]}
                onPress={action.onPress}
              >
                <MaterialIcons
                  name={action.icon}
                  size={20}
                  color={action.color}
                  style={styles.actionIcon}
                />
                <Text style={[styles.actionLabel, { color: action.color }]}>
                  {action.label}
                </Text>
              </TouchableOpacity>
            ))}
          </View>
        </View>
      </TouchableOpacity>
    </Modal>
  );
}

const styles = StyleSheet.create({
  overlay: {
    flex: 1,
    backgroundColor: 'rgba(0, 0, 0, 0.4)',
    justifyContent: 'center',
    alignItems: 'center',
    padding: 20,
  },
  container: {
    borderRadius: 12,
    minWidth: 240,
    maxWidth: 300,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 4 },
    shadowOpacity: 0.2,
    shadowRadius: 8,
    elevation: 8,
    overflow: 'hidden',
  },
  reactionsRow: {
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
    paddingVertical: 10,
    paddingHorizontal: 8,
    gap: 4,
  },
  emojiButton: {
    padding: 6,
    borderRadius: 16,
    backgroundColor: 'rgba(0, 0, 0, 0.04)',
  },
  emoji: {
    fontSize: 22,
  },
  divider: {
    height: 1,
    marginHorizontal: 12,
  },
  actionsList: {
    paddingVertical: 4,
  },
  actionRow: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 10,
    paddingHorizontal: 16,
  },
  actionBorder: {
    borderBottomWidth: 0.5,
  },
  actionIcon: {
    width: 24,
    alignItems: 'center',
    justifyContent: 'center',
  },
  actionLabel: {
    fontSize: 14,
    fontWeight: '500',
    marginLeft: 12,
  },
});

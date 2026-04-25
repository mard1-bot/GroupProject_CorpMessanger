import React from 'react';
import { View, Text, TouchableOpacity, StyleSheet } from 'react-native';
import { MaterialIcons } from '@expo/vector-icons';
import { useThemeColor } from '@/hooks/use-theme-color';
import { Reaction } from '@/types/chat';

interface MessageReactionsProps {
  reactions: Reaction[];
  compact?: boolean;
}

export function MessageReactions({ reactions, compact = false }: MessageReactionsProps) {
  const textColor = useThemeColor({}, 'text');
  const primaryColor = useThemeColor({}, 'primary');
  const surfaceColor = useThemeColor({}, 'surface');

  if (!reactions || reactions.length === 0) {
    return null;
  }

  // Group reactions by emoji
  const groupedReactions = reactions.reduce((acc, reaction) => {
    if (!acc[reaction.emoji]) {
      acc[reaction.emoji] = [];
    }
    acc[reaction.emoji].push(reaction);
    return acc;
  }, {} as Record<string, Reaction[]>);

  const reactionEntries = Object.entries(groupedReactions);

  return (
    <View style={[styles.container, compact && styles.containerCompact]}>
      {reactionEntries.map(([emoji, reactionList]) => {
        const count = reactionList.length;
        const isOwnReaction = reactionList.some(r => r.user_id === 'current-user-id'); // TODO: Get current user ID
        
        return (
          <TouchableOpacity
            key={emoji}
            style={[
              styles.reactionBubble,
              compact && styles.reactionBubbleCompact,
              isOwnReaction && { backgroundColor: primaryColor + '20', borderWidth: 1, borderColor: primaryColor }
            ]}
            onPress={() => {
              // TODO: Toggle reaction
            }}
          >
            <Text style={[styles.reactionEmoji, compact && styles.reactionEmojiCompact]}>
              {emoji}
            </Text>
            {!compact && count > 1 && (
              <Text style={[styles.reactionCount, { color: textColor }]}>
                {count}
              </Text>
            )}
          </TouchableOpacity>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flexDirection: 'row',
    alignItems: 'center',
    flexWrap: 'wrap',
    marginTop: 4,
    gap: 4,
  },
  containerCompact: {
    marginTop: 2,
  },
  addButton: {
    padding: 4,
    borderRadius: 12,
    backgroundColor: 'rgba(0, 0, 0, 0.05)',
    justifyContent: 'center',
    alignItems: 'center',
    minWidth: 24,
    height: 24,
  },
  addButtonCompact: {
    minWidth: 20,
    height: 20,
  },
  reactionBubble: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 2,
    paddingHorizontal: 6,
    borderRadius: 12,
    backgroundColor: 'rgba(0, 0, 0, 0.05)',
    minWidth: 24,
    height: 24,
  },
  reactionBubbleCompact: {
    minWidth: 20,
    height: 20,
    paddingHorizontal: 4,
  },
  reactionEmoji: {
    fontSize: 14,
  },
  reactionEmojiCompact: {
    fontSize: 12,
  },
  reactionCount: {
    fontSize: 11,
    marginLeft: 2,
    fontWeight: '500',
  },
});

import React from 'react';
import { Text, TextStyle, StyleSheet } from 'react-native';
import { useRouter } from 'expo-router';

interface MentionTextProps {
  text: string;
  style?: TextStyle | TextStyle[];
  membersMap?: Record<string, { username?: string; id: string }>;
}

export function MentionText({ text, style, membersMap }: MentionTextProps) {
  const router = useRouter();

  // Build username -> userId map from membersMap
  const usernameMap = React.useMemo(() => {
    const map: Record<string, string> = {};
    if (membersMap) {
      for (const user of Object.values(membersMap)) {
        if (user.username) {
          map[user.username.toLowerCase()] = user.id;
        }
      }
    }
    return map;
  }, [membersMap]);

  const handleMentionPress = (username: string) => {
    const userId = usernameMap[username.toLowerCase()];
    if (userId) {
      router.push(`/user/${userId}`);
    }
  };

  // Split text into parts: @username and regular text
  const parts = React.useMemo(() => {
    const regex = /@(\w{3,50})/g;
    const result: Array<{ type: 'text' | 'mention'; value: string }> = [];
    let lastIndex = 0;
    let match;

    while ((match = regex.exec(text)) !== null) {
      // Text before the mention
      if (match.index > lastIndex) {
        result.push({ type: 'text', value: text.slice(lastIndex, match.index) });
      }
      // The @username mention
      const username = match[1];
      const isValidMention = usernameMap[username.toLowerCase()] !== undefined;
      if (isValidMention) {
        result.push({ type: 'mention', value: username });
      } else {
        // Not a valid username, render as plain text
        result.push({ type: 'text', value: match[0] });
      }
      lastIndex = regex.lastIndex;
    }

    // Remaining text after last mention
    if (lastIndex < text.length) {
      result.push({ type: 'text', value: text.slice(lastIndex) });
    }

    return result;
  }, [text, usernameMap]);

  if (parts.length === 0) {
    return <Text style={style}>{text}</Text>;
  }

  const flatStyle = StyleSheet.flatten(style);

  return (
    <Text style={flatStyle}>
      {parts.map((part, index) => {
        if (part.type === 'mention') {
          return (
            <Text
              key={index}
              style={{ color: '#2196F3', fontWeight: '600' }}
              onPress={() => handleMentionPress(part.value)}
            >
              {'@' + part.value}
            </Text>
          );
        }
        return <Text key={index}>{part.value}</Text>;
      })}
    </Text>
  );
}

import React from 'react';
import { View, Text, StyleSheet, TouchableOpacity, Modal, Dimensions } from 'react-native';
import { MaterialIcons } from '@expo/vector-icons';
import { callService, type CallState } from '@/services/calls';

interface IncomingCallProps {
  callState: CallState;
  callerName: string;
  isGroup?: boolean;
  onAccept: () => void;
  onReject: () => void;
}

export function IncomingCall({ callState, callerName, isGroup, onAccept, onReject }: IncomingCallProps) {
  const [isProcessing, setIsProcessing] = React.useState(false);
  const handleAcceptPress = async () => {
    if (isProcessing) return;
    setIsProcessing(true);
    console.log('Accept button pressed');
    try {
      await onAccept();
    } finally {
      setIsProcessing(false);
    }
  };

  const handleRejectPress = async () => {
    if (isProcessing) return;
    setIsProcessing(true);
    console.log('Reject button pressed');
    try {
      await onReject();
    } finally {
      setIsProcessing(false);
    }
  };

  return (
    <Modal visible={true} transparent animationType="slide">
      <View style={styles.overlay}>
        <View style={styles.container}>
          <View style={styles.avatar}>
            <MaterialIcons name={isGroup ? 'group' : 'person'} size={80} color="#666" />
          </View>
          
          <Text style={styles.callerName}>{callerName}</Text>
          <Text style={styles.callType}>
            {callState.type === 'video' ? 'Видеозвонок' : 'Аудиозвонок'}
          </Text>
          <Text style={styles.status}>Входящий звонок...</Text>

          <View style={styles.buttons}>
            <TouchableOpacity 
              style={[styles.button, styles.rejectButton]}
              onPress={handleRejectPress}
              disabled={isProcessing}
              activeOpacity={0.7}
              hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}
            >
              <MaterialIcons name="call-end" size={32} color={isProcessing ? "#ccc" : "#fff"} />
              <Text style={styles.buttonText} numberOfLines={1}>Отклонить</Text>
            </TouchableOpacity>

            <TouchableOpacity 
              style={[styles.button, styles.acceptButton]}
              onPress={handleAcceptPress}
              disabled={isProcessing}
              activeOpacity={0.7}
              hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}
            >
              <MaterialIcons name="call" size={32} color={isProcessing ? "#ccc" : "#fff"} />
              <Text style={styles.buttonText} numberOfLines={1}>Ответить</Text>
            </TouchableOpacity>
          </View>
        </View>
      </View>
    </Modal>
  );
}

const { width } = Dimensions.get('window');

const styles = StyleSheet.create({
  overlay: {
    flex: 1,
    backgroundColor: 'rgba(0,0,0,0.9)',
    justifyContent: 'center',
    alignItems: 'center',
  },
  container: {
    alignItems: 'center',
    padding: 40,
  },
  avatar: {
    width: 120,
    height: 120,
    borderRadius: 60,
    backgroundColor: '#333',
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: 30,
  },
  callerName: {
    fontSize: 28,
    fontWeight: '600',
    color: '#fff',
    marginBottom: 8,
  },
  callType: {
    fontSize: 18,
    color: '#aaa',
    marginBottom: 4,
  },
  status: {
    fontSize: 16,
    color: '#888',
    marginBottom: 50,
  },
  buttons: {
    flexDirection: 'row',
    gap: 50,
  },
  button: {
    alignItems: 'center',
    gap: 8,
    minWidth: 80,
    minHeight: 80,
    justifyContent: 'center',
  },
  buttonText: {
    color: '#fff',
    fontSize: 14,
  },
  rejectButton: {
    width: 80,
    height: 80,
    borderRadius: 40,
    backgroundColor: '#e53935',
    justifyContent: 'center',
    alignItems: 'center',
  },
  acceptButton: {
    width: 80,
    height: 80,
    borderRadius: 40,
    backgroundColor: '#43a047',
    justifyContent: 'center',
    alignItems: 'center',
  },
});

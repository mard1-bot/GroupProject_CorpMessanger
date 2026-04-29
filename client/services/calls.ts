import { Platform } from 'react-native';
import { webCallService, type CallType, type CallState } from './webrtc-web';

// Platform detection - use a more reliable check
const isWeb = typeof window !== 'undefined' && typeof document !== 'undefined';

// Platform-specific service
let platformService: any = webCallService;

// Only try to load native module on non-web platforms
if (!isWeb && Platform.OS !== 'web') {
  try {
    // Use dynamic import to avoid bundling react-native-webrtc on web
    const nativeModule = require('./webrtc-native');
    if (nativeModule && nativeModule.nativeCallService) {
      platformService = nativeModule.nativeCallService;
    }
  } catch (e) {
    console.warn('[Calls] Native WebRTC not available, using web implementation');
  }
}

// Re-export all methods from platform service
export const callService = platformService;

// Re-export types for type checking
export type { CallType, CallState } from './webrtc-web';

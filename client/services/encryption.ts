import AsyncStorage from '@react-native-async-storage/async-storage';
import { Platform } from 'react-native';
import { API_URL } from './api';

const ENCRYPTION_KEYS_STORAGE = '@encryption_keys';
const CHAT_KEYS_STORAGE = '@chat_keys';

export interface EncryptionKeyPair {
  publicKey: string;
  privateKey: string;
  version: number;
}

export interface UserPublicKey {
  user_id: string;
  public_key: string;
  key_version: number;
  created_at: string;
}

export interface EncryptedMessage {
  encryptedContent: string;
  encryptedKeys: Record<string, string>; // user_id -> encrypted session key
  iv: string;
}

function arrayBufferToBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = '';
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

function base64ToArrayBuffer(base64: string): ArrayBuffer {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer;
}

class EncryptionService {
  private keyPair: EncryptionKeyPair | null = null;
  private chatPublicKeys: Map<string, Map<string, UserPublicKey>> = new Map();

  async initialize(): Promise<void> {
    await this.loadKeys();
  }

  // Generate ECDH key pair using Web Crypto API
  async generateKeyPair(): Promise<EncryptionKeyPair> {
    // Use ECDH with P-256 curve (widely supported across platforms)
    const keyPair = await crypto.subtle.generateKey(
      {
        name: 'ECDH',
        namedCurve: 'P-256',
      },
      true, // extractable
      ['deriveBits']
    ) as CryptoKeyPair;

    const publicKeyJwk = await crypto.subtle.exportKey('jwk', keyPair.publicKey);
    const privateKeyJwk = await crypto.subtle.exportKey('jwk', keyPair.privateKey);

    return {
      publicKey: JSON.stringify(publicKeyJwk),
      privateKey: JSON.stringify(privateKeyJwk),
      version: 1,
    };
  }

  async registerPublicKey(token: string, publicKey: string, privateKey: string): Promise<void> {
    const response = await fetch(`${API_URL}/api/v1/encryption/keys`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
      },
      body: JSON.stringify({
        public_key: publicKey,
        private_key: privateKey,
      }),
    });

    if (!response.ok) {
      throw new Error('Failed to register public key');
    }
  }

  async getMyPublicKey(token: string): Promise<UserPublicKey | null> {
    const response = await fetch(`${API_URL}/api/v1/encryption/keys/me`, {
      headers: {
        'Authorization': `Bearer ${token}`,
      },
    });

    if (response.status === 404) {
      return null;
    }

    if (!response.ok) {
      throw new Error('Failed to get public key');
    }

    return await response.json();
  }

  async getChatPublicKeys(token: string, chatId: string): Promise<UserPublicKey[]> {
    // Check cache first
    const cached = this.chatPublicKeys.get(chatId);
    if (cached) {
      return Array.from(cached.values());
    }

    const response = await fetch(
      `${API_URL}/api/v1/encryption/keys/chat?chat_id=${chatId}`,
      {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      }
    );

    if (!response.ok) {
      throw new Error('Failed to get chat public keys');
    }

    const data = await response.json();
    const keys = data.keys as UserPublicKey[];

    // Cache the keys
    const keyMap = new Map<string, UserPublicKey>();
    keys.forEach((key) => {
      keyMap.set(key.user_id, key);
    });
    this.chatPublicKeys.set(chatId, keyMap);

    return keys;
  }

  async getUserPublicKey(
    token: string,
    userId: string,
    chatId?: string
  ): Promise<UserPublicKey | null> {
    // Check cache first
    if (chatId) {
      const cached = this.chatPublicKeys.get(chatId);
      if (cached) {
        const key = cached.get(userId);
        if (key) return key;
      }
    }

    const response = await fetch(
      `${API_URL}/api/v1/encryption/keys/user?user_id=${userId}`,
      {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      }
    );

    if (response.status === 404) {
      return null;
    }

    if (!response.ok) {
      throw new Error('Failed to get user public key');
    }

    return await response.json();
  }

  // Load keys from secure storage
  private async loadKeys(): Promise<void> {
    try {
      const keysJson = await AsyncStorage.getItem(ENCRYPTION_KEYS_STORAGE);
      if (keysJson) {
        this.keyPair = JSON.parse(keysJson);
      }
    } catch (error) {
      console.error('Failed to load encryption keys:', error);
    }
  }

  // Save keys to secure storage
  private async saveKeys(): Promise<void> {
    if (this.keyPair) {
      await AsyncStorage.setItem(
        ENCRYPTION_KEYS_STORAGE,
        JSON.stringify(this.keyPair)
      );
    }
  }

  // Get or create key pair
  async getOrCreateKeyPair(token: string): Promise<EncryptionKeyPair> {
    if (this.keyPair) {
      return this.keyPair;
    }

    // Check if keys exist on server
    const serverKey = await this.getMyPublicKey(token);
    
    if (serverKey) {
      // Load private key from storage
      await this.loadKeys();
      if (this.keyPair) {
        return this.keyPair;
      }
    }

    // Generate new keys
    this.keyPair = await this.generateKeyPair();
    await this.saveKeys();

    // Register on server
    await this.registerPublicKey(
      token,
      this.keyPair.publicKey,
      this.keyPair.privateKey
    );

    return this.keyPair;
  }

  // Derive shared secret using ECDH
  private async deriveSharedSecret(
    privateKeyJwk: JsonWebKey,
    publicKeyJwk: JsonWebKey
  ): Promise<ArrayBuffer> {
    const privateKey = await crypto.subtle.importKey(
      'jwk',
      privateKeyJwk,
      { name: 'ECDH', namedCurve: 'P-256' },
      false,
      ['deriveBits']
    );

    const publicKey = await crypto.subtle.importKey(
      'jwk',
      publicKeyJwk,
      { name: 'ECDH', namedCurve: 'P-256' },
      true,
      []
    );

    // Derive 256-bit shared secret
    return await crypto.subtle.deriveBits(
      { name: 'ECDH', public: publicKey },
      privateKey,
      256
    );
  }

  // Encrypt message for multiple recipients (group chat)
  async encryptMessage(
    token: string,
    chatId: string,
    content: string
  ): Promise<EncryptedMessage> {
    // Get public keys for all chat members
    const publicKeys = await this.getChatPublicKeys(token, chatId);
    
    if (publicKeys.length === 0) {
      throw new Error('No public keys available for encryption');
    }

    if (!this.keyPair) {
      throw new Error('No local key pair available');
    }

    // Generate random AES-256-GCM key for message encryption
    const sessionKey = crypto.getRandomValues(new Uint8Array(32));
    const iv = crypto.getRandomValues(new Uint8Array(12));

    // Import session key for AES-GCM
    const aesKey = await crypto.subtle.importKey(
      'raw',
      sessionKey,
      { name: 'AES-GCM' },
      false,
      ['encrypt']
    );

    // Encrypt content with session key
    const encrypted = await crypto.subtle.encrypt(
      { name: 'AES-GCM', iv },
      aesKey,
      new TextEncoder().encode(content)
    );

    const encryptedContent = arrayBufferToBase64(encrypted);

    // Encrypt session key for each recipient using ECDH
    const encryptedKeys: Record<string, string> = {};
    const privateKeyJwk = JSON.parse(this.keyPair.privateKey) as JsonWebKey;

    for (const userKey of publicKeys) {
      try {
        const recipientPublicKeyJwk = JSON.parse(userKey.public_key) as JsonWebKey;
        
        // Derive shared secret with recipient
        const sharedSecret = await this.deriveSharedSecret(
          privateKeyJwk,
          recipientPublicKeyJwk
        );

        // Use shared secret to encrypt session key with AES-GCM
        const keyEncIv = crypto.getRandomValues(new Uint8Array(12));
        const wrappingKey = await crypto.subtle.importKey(
          'raw',
          sharedSecret,
          { name: 'AES-GCM' },
          false,
          ['encrypt']
        );

        const encryptedSessionKey = await crypto.subtle.encrypt(
          { name: 'AES-GCM', iv: keyEncIv },
          wrappingKey,
          sessionKey
        );

        // Combine iv + encrypted session key
        const combined = new Uint8Array(12 + encryptedSessionKey.byteLength);
        combined.set(keyEncIv, 0);
        combined.set(new Uint8Array(encryptedSessionKey), 12);

        encryptedKeys[userKey.user_id] = arrayBufferToBase64(combined.buffer);
      } catch (error) {
        console.error(`Failed to encrypt key for user ${userKey.user_id}:`, error);
        // Skip this recipient - they won't be able to decrypt
      }
    }

    return {
      encryptedContent,
      encryptedKeys,
      iv: arrayBufferToBase64(iv.buffer),
    };
  }

  // Decrypt message
  async decryptMessage(
    encryptedMessage: EncryptedMessage,
    currentUserId: string,
    senderId?: string
  ): Promise<string> {
    if (!this.keyPair) {
      throw new Error('No encryption keys available');
    }

    // Get encrypted session key for current user
    const encryptedKeyData = encryptedMessage.encryptedKeys[currentUserId];
    if (!encryptedKeyData) {
      throw new Error('No session key available for this user');
    }

    // Parse combined iv + encrypted session key
    const combined = new Uint8Array(base64ToArrayBuffer(encryptedKeyData));
    const keyEncIv = combined.slice(0, 12);
    const encryptedSessionKey = combined.slice(12);

    // We need the sender's public key to derive the shared secret
    const privateKeyJwk = JSON.parse(this.keyPair.privateKey) as JsonWebKey;
    let sessionKey: Uint8Array | null = null;

    if (senderId) {
      // If sender ID is provided, only try that specific key
      for (const [chatId, keyMap] of this.chatPublicKeys) {
        const userKey = keyMap.get(senderId);
        if (userKey) {
          try {
            const senderPublicKeyJwk = JSON.parse(userKey.public_key) as JsonWebKey;
            
            const sharedSecret = await this.deriveSharedSecret(
              privateKeyJwk,
              senderPublicKeyJwk
            );

            const wrappingKey = await crypto.subtle.importKey(
              'raw',
              sharedSecret,
              { name: 'AES-GCM' },
              false,
              ['decrypt']
            );

            const decrypted = await crypto.subtle.decrypt(
              { name: 'AES-GCM', iv: keyEncIv },
              wrappingKey,
              encryptedSessionKey
            );

            sessionKey = new Uint8Array(decrypted);
            break;
          } catch (error) {
            console.error(`Failed to decrypt with sender ${senderId}:`, error);
          }
        }
      }
    } else {
      // Fallback: try each cached public key (less efficient, for backward compatibility)
      for (const [chatId, keyMap] of this.chatPublicKeys) {
        for (const [userId, userKey] of keyMap) {
          try {
            const senderPublicKeyJwk = JSON.parse(userKey.public_key) as JsonWebKey;
            
            const sharedSecret = await this.deriveSharedSecret(
              privateKeyJwk,
              senderPublicKeyJwk
            );

            const wrappingKey = await crypto.subtle.importKey(
              'raw',
              sharedSecret,
              { name: 'AES-GCM' },
              false,
              ['decrypt']
            );

            const decrypted = await crypto.subtle.decrypt(
              { name: 'AES-GCM', iv: keyEncIv },
              wrappingKey,
              encryptedSessionKey
            );

            sessionKey = new Uint8Array(decrypted);
            break;
          } catch {
            // Not the right key, continue
          }
        }
        if (sessionKey) break;
      }
    }

    if (!sessionKey) {
      throw new Error('Failed to decrypt session key - sender public key may not be cached');
    }

    // Decrypt message content with session key
    const iv = new Uint8Array(base64ToArrayBuffer(encryptedMessage.iv));
    const aesKey = await crypto.subtle.importKey(
      'raw',
      sessionKey.buffer as ArrayBuffer,
      { name: 'AES-GCM' },
      false,
      ['decrypt']
    );

    const encrypted = base64ToArrayBuffer(encryptedMessage.encryptedContent);
    
    const decrypted = await crypto.subtle.decrypt(
      { name: 'AES-GCM', iv },
      aesKey,
      encrypted
    );

    return new TextDecoder().decode(decrypted);
  }

  // Check if E2E encryption is available
  isEncryptionAvailable(): boolean {
    return this.keyPair !== null;
  }

  // Clear cached keys (e.g., on logout)
  async clearKeys(): Promise<void> {
    this.keyPair = null;
    this.chatPublicKeys.clear();
    await AsyncStorage.removeItem(ENCRYPTION_KEYS_STORAGE);
    await AsyncStorage.removeItem(CHAT_KEYS_STORAGE);
  }
}

export const encryptionService = new EncryptionService();

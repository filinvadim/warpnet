import base64 from 'base64-js';
import diffieHellman from 'diffie-hellman';
import { Buffer } from 'buffer';

// i hate frontend
export default class EncryptedSocketClient {
    constructor(url) {
        this.dh = diffieHellman.getDiffieHellman('modp14'); // Используем группу modp14 (2048 бит)
        this.dh.generateKeys();
        this.url = url;
        this.publicKey = this.dh.getPublicKey();
        this.externalPublicKey = null
        this.aesKey = null; // Will be derived after receiving server's public key
        this.callback = null;
        this.socket = null
        this.secretRenew = false;
    }

    // Helper function to split message into ciphertext and nonce
    splitMessage(message) {
        let parts = message.split(":");
        if (parts.length !== 2) {
            console.error('Invalid message format:', message);
        }
        return parts
    }

    async deriveKey(sharedSecret, salt) {
        const keyMaterial = await crypto.subtle.importKey(
            'raw',
            sharedSecret,
            'HKDF',
            false,
            ['deriveKey']
        );

        return crypto.subtle.deriveKey(
            {
                name: 'HKDF',
                salt: new TextEncoder().encode(salt),
                info: new Uint8Array(),
                hash: 'SHA-256',
            },
            keyMaterial,
            { name: 'AES-GCM', length: 256 },
            true,
            ['encrypt', 'decrypt']
        );
    }

    async encryptMessage(secretKey, plaintext) {
        const iv = crypto.getRandomValues(new Uint8Array(12)); // 96-bit nonce
        const ciphertext = await crypto.subtle.encrypt(
            {
                name: 'AES-GCM',
                iv: iv,
            },
            secretKey,
            new TextEncoder().encode(plaintext)
        );

        return {
            ciphertext: Buffer.from(new Uint8Array(ciphertext)),
            nonce: Buffer.from(iv),
        };
    }

    async decryptMessage(secretKey, ciphertext, nonce) {
        const decrypted = await crypto.subtle.decrypt(
            {
                name: 'AES-GCM',
                iv: nonce,
            },
            secretKey,
            ciphertext
        );

        return new TextDecoder().decode(decrypted);
    }

    async onMessage(event) {
        console.log('Message received:', event.data);
        const message = typeof event.data === "string" ? event.data : JSON.stringify(event.data);
        const receivedMessage = Buffer.from(message, "base64");

        // Handle the server's public key (first message)
        if (!this.aesKey) {
            console.log('Received server public key');
            this.externalPublicKey = receivedMessage
            // Compute shared secret
            await this.computeSecret('warpnet') // temp salt
            console.log('Shared secret established.');
            return;
        }

        // Process encrypted messages
        const [ciphertextBase64, nonceBase64] = this.splitMessage(message);
        const ciphertext = Buffer.from(ciphertextBase64, 'base64');
        const nonce = Buffer.from(nonceBase64, 'base64');

        // Decrypt the message
        let plaintext = ''
        try {
            plaintext = await this.decryptMessage(this.aesKey, ciphertext, nonce);
        } catch (err) {
            console.error('Error decrypting message:', err.message);
            return;
        }

        if (!this.callback) {
            console.log('No callback provided.');
            return;
        }

        let data = null
        try {
            data = JSON.parse(plaintext);
        } catch (error) {
            console.error("JSON parsing:", error.message);
            this.socket.send(error.message)
            return;
        }
        if (!data) {
            return;
        }

        if (data.token && !this.secretRenew) {
            await this.computeSecret(data.token); // renew salt to more secure one
            this.secretRenew = true;
            console.log('secret renewed')
        }

        const response = this.callback(data);
        if (!response) {
            return;
        }

        const {
            ciphertext: encryptedResponse,
            nonce: responseNonce
        } = await this.encryptMessage(this.aesKey, response);
        const payload = `${base64.fromByteArray(encryptedResponse)}:${base64.fromByteArray(responseNonce)}`;

        this.socket.send(payload);
    }

    onError(error) {
        console.error('WebSocket error:', error);
    }

    onClose() {
        console.log('WebSocket connection closed');
    }

    close() {
        if (this.socket) {
            this.socket.removeEventListener('open', this.onOpen.bind(this));
            this.socket.removeEventListener('close', this.onClose.bind(this));
            this.socket.removeEventListener('message', this.onMessage.bind(this));
            this.socket.removeEventListener('error', this.onError.bind(this));
            this.socket.close();
            this.socket = null;
        }
    }

    async computeSecret(salt) {
        const sharedSecret = this.dh.computeSecret(this.externalPublicKey);
        // Derive AES key from the shared secret
        this.aesKey = await this.deriveKey(sharedSecret, salt);
    }

    connect() {
        console.log("connect called");
        try {
            this.socket = new WebSocket(this.url);
            this.socket.addEventListener('open', this.onOpen.bind(this));
            this.socket.addEventListener('close', this.onClose.bind(this));
            this.socket.addEventListener('message', this.onMessage.bind(this));
            this.socket.addEventListener('error', this.onError.bind(this));
        } catch (error) {
            console.error("Error while connecting WebSocket:", error);
        }
    }

    onOpen() {
        console.log('WebSocket connection opened');
        // Отправляем публичный ключ
        this.socket.send(this.publicKey.toString('base64'));
        console.log('Public key is sent.');
    }


    setupCallback(callback) {
        this.callback = callback;
    }

    async sendMessage(message) {
        message = JSON.stringify(message)
        if (!this.aesKey) {
            console.log("Sending message: not authenticated!");
            throw new Error("Not authenticated!");
        }
        if (this.socket.readyState !== WebSocket.OPEN) {
            console.error('WebSocket is not open');
            return;
        }
        const {
            ciphertext: encryptedResponse,
            nonce: responseNonce
        } = await this.encryptMessage(this.aesKey, message);
        const payload = `${base64.fromByteArray(encryptedResponse)}:${base64.fromByteArray(responseNonce)}`;

        await this.socket.send(payload);
    }
}

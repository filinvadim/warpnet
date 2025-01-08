import { createLibp2p } from 'libp2p';
import { tcp } from '@libp2p/tcp';
import { noise } from '@chainsafe/libp2p-noise';
import { yamux } from '@chainsafe/libp2p-yamux';
import { pipe } from 'it-pipe';

let backendMultiAddr = ""

async function createClient() {
    const node = await createLibp2p({
        transports: [tcp()],
        connectionEncryption: [noise()],
        streamMuxers: [yamux()],
    });

    await node.start();
    console.log('libp2p client started:', node.peerId.toString());

    return node;
}

async function sendRequest(node: Libp2p, message: string) {
    if (backendMultiAddr === "") {
        console.log("Empty backend address");
        return;
    }
    console.log(`Connecting to peer: ${backendMultiAddr}`);

    const connection = await node.dial(peerAddress);
    console.log('Connected to peer:', peerAddress);

    const { stream } = await connection.newStream(['/my-protocol']);
    await pipe(
        [message],
        stream,
        async (source) => {
            for await (const chunk of source) {
                console.log('Received response:', new TextDecoder().decode(chunk));
            }
        }
    );
}

(async () => {
    const client = await createClient();

    const peerAddress = '/ip4/127.0.0.1/tcp/8080/p2p/QmExamplePeerId';

    const message = 'Hello from libp2p client!';
    await sendRequest(client, peerAddress, message);

    await client.stop();
    console.log('Client stopped');
})();

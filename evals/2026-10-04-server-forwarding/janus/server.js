import { JanusWhipServer } from 'janus-whip-server'
import { JanusWhepServer } from 'janus-whep-server'
// WHIP publishes into VideoRoom 1234 and forwards the video over RTP to a Streaming
// mountpoint, one per codec; WHEP subscribes to the mountpoint with a client offer.
const janus = { address: 'ws://127.0.0.1:8188' }
const whip = new JanusWhipServer({ janus, rest: { port: 7080, basePath: '/whip' } })
await whip.start()
whip.createEndpoint({ id: 'vp8', room: 1234, label: 'whipbench-vp8', recipient: { host: '127.0.0.1', videoPort: 5004 } })
whip.createEndpoint({ id: 'h264', room: 1234, label: 'whipbench-h264', recipient: { host: '127.0.0.1', videoPort: 5006 } })
const whep = new JanusWhepServer({ janus, rest: { port: 7090, basePath: '/whep' } })
await whep.start()
whep.createEndpoint({ id: 'vp8', plugin: 'streaming', mountpoint: 1 })
whep.createEndpoint({ id: 'h264', plugin: 'streaming', mountpoint: 2 })
console.log('whip and whep ready')

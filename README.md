# heartbeat
Heartbeat is helper library for [reliable](https://github.com/lithdew/reliable).

## Features
### Hertbeat
Send heartbeat packets to be asked by a receiver. A receiver ack every 32 packets.<br>
heartbeat packets sending start when `WritePacket` method is called. Sending continue until deadline come. The deadline is reset whenever `WritePacket` is called. The deadline and heartbeat period can be set through option.

### Fragmentation
Fragment packets to send large size of packet.<br>
Any packets which exceeds `fragment size` is divided into smaller ones. A receiver reassemble fragmented packets. The fragment size and max fragments can be set through option.

## Notes
- Send packet via `WriteReliablePacket` of `reliable` which mean that stop sending if a receiver don't ack.
- `reliable.WithEndpointPacketHandler` is overwritten.
- Allowed max packet size can be sent is `MaxUint32(=4294967295)`.
- Allowed max fragmentation size is `MaxUint8(=255)`.

## Example
```go
func logPacket(packetData []byte) {
  log.Printf("recv (packetData size=%d)", len(packetData))
}

func logErr(err error) {
  log.Panic(err)
}

ca, _ := net.ListenPacket("udp", "127.0.0.1:0")
cb, _ := net.ListenPacket("udp", "127.0.0.1:0")

e, _ := heartbeat.NewEndpoint(ca, logPacket, logErr, nil)

e.WritePacket([]byte("hello world"), cb.LocalAddr())
```

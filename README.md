# heartbeat

## Important nones
- Allowed max packet size can be sent is `MaxUint32(4294967296)`.
- A sent data which is same as heartbeat packet(`=[]byte(".hb")`) is ignored.

package heartbeat

func searchUint8(key uint8, s []uint8) (int, bool) {
	for i := range s {
		if s[i] == key {
			return i, true
		}
	}
	return 0, false
}

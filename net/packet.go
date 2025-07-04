package net

type Packet struct {
	Length uint16
	Packet [1420]byte
}

package protocol

import (
	"fmt"
)

// DMRData holds DMR frame information equivalent to C++ CDMRData
type DMRData struct {
	SlotNo    uint8    // DMR slot number (1 or 2)
	SrcId     uint32   // 24-bit source ID
	DstId     uint32   // 24-bit destination ID
	FLCO      uint8    // Frame Level Call type (FLCO_GROUP, FLCO_USER_USER, etc.)
	DataType  uint8    // Data type (DT_VOICE, DT_VOICE_SYNC, etc.)
	N         uint8    // Voice frame counter (0-5) or other N value
	SeqNo     uint8    // Sequence number
	Data      [33]byte // 33-byte DMR payload
	BER       uint8    // Bit Error Rate
	RSSI      uint8    // Received Signal Strength Indicator
	StreamId  uint32   // Per-slot stream identifier
	Missing   bool     // Frame loss indicator
}

// NewDMRData creates a new DMRData instance
func NewDMRData() *DMRData {
	return &DMRData{}
}

// GetSlotNo returns the slot number
func (d *DMRData) GetSlotNo() uint8 {
	return d.SlotNo
}

// SetSlotNo sets the slot number
func (d *DMRData) SetSlotNo(slotNo uint8) {
	d.SlotNo = slotNo
}

// GetSrcId returns the source ID
func (d *DMRData) GetSrcId() uint32 {
	return d.SrcId
}

// SetSrcId sets the source ID (24-bit)
func (d *DMRData) SetSrcId(srcId uint32) {
	d.SrcId = srcId & 0xFFFFFF // Mask to 24 bits
}

// GetDstId returns the destination ID
func (d *DMRData) GetDstId() uint32 {
	return d.DstId
}

// SetDstId sets the destination ID (24-bit)
func (d *DMRData) SetDstId(dstId uint32) {
	d.DstId = dstId & 0xFFFFFF // Mask to 24 bits
}

// GetFLCO returns the Frame Level Call type
func (d *DMRData) GetFLCO() uint8 {
	return d.FLCO
}

// SetFLCO sets the Frame Level Call type
func (d *DMRData) SetFLCO(flco uint8) {
	d.FLCO = flco
}

// GetDataType returns the data type
func (d *DMRData) GetDataType() uint8 {
	return d.DataType
}

// SetDataType sets the data type
func (d *DMRData) SetDataType(dataType uint8) {
	d.DataType = dataType
}

// GetN returns the voice frame counter or N value
func (d *DMRData) GetN() uint8 {
	return d.N
}

// SetN sets the voice frame counter or N value
func (d *DMRData) SetN(n uint8) {
	d.N = n
}

// GetSeqNo returns the sequence number
func (d *DMRData) GetSeqNo() uint8 {
	return d.SeqNo
}

// SetSeqNo sets the sequence number
func (d *DMRData) SetSeqNo(seqNo uint8) {
	d.SeqNo = seqNo
}

// GetData returns a copy of the data array
func (d *DMRData) GetData() [33]byte {
	return d.Data
}

// SetData sets the data array
func (d *DMRData) SetData(data []byte) {
	copy(d.Data[:], data)
}

// GetDataPtr returns a pointer to the data array for direct manipulation
func (d *DMRData) GetDataPtr() []byte {
	return d.Data[:]
}

// GetBER returns the Bit Error Rate
func (d *DMRData) GetBER() uint8 {
	return d.BER
}

// SetBER sets the Bit Error Rate
func (d *DMRData) SetBER(ber uint8) {
	d.BER = ber
}

// GetRSSI returns the Received Signal Strength Indicator
func (d *DMRData) GetRSSI() uint8 {
	return d.RSSI
}

// SetRSSI sets the Received Signal Strength Indicator
func (d *DMRData) SetRSSI(rssi uint8) {
	d.RSSI = rssi
}

// GetStreamId returns the stream identifier
func (d *DMRData) GetStreamId() uint32 {
	return d.StreamId
}

// SetStreamId sets the stream identifier
func (d *DMRData) SetStreamId(streamId uint32) {
	d.StreamId = streamId
}

// IsMissing returns true if this frame is marked as missing
func (d *DMRData) IsMissing() bool {
	return d.Missing
}

// SetMissing sets the missing flag
func (d *DMRData) SetMissing(missing bool) {
	d.Missing = missing
}

// IsDataSync returns true if data sync flag is set
func (d *DMRData) IsDataSync() bool {
	// Data sync is indicated by specific data types
	return d.DataType == DT_DATA_HEADER ||
		   d.DataType == DT_RATE_12_DATA ||
		   d.DataType == DT_RATE_34_DATA ||
		   d.DataType == DT_RATE_1_DATA
}

// IsVoiceSync returns true if voice sync flag is set
func (d *DMRData) IsVoiceSync() bool {
	return d.DataType == DT_VOICE_SYNC
}

// IsVoice returns true if this is a voice frame
func (d *DMRData) IsVoice() bool {
	return d.DataType == DT_VOICE || d.DataType == DT_VOICE_SYNC
}

// IsVoiceLCHeader returns true if this is a voice LC header
func (d *DMRData) IsVoiceLCHeader() bool {
	return d.DataType == DT_VOICE_LC_HEADER
}

// IsTerminator returns true if this is a terminator frame
func (d *DMRData) IsTerminator() bool {
	return d.DataType == DT_TERMINATOR_WITH_LC
}

// IsGroupCall returns true if this is a group call
func (d *DMRData) IsGroupCall() bool {
	return d.FLCO == FLCO_GROUP
}

// IsPrivateCall returns true if this is a private call
func (d *DMRData) IsPrivateCall() bool {
	return d.FLCO == FLCO_USER_USER
}

// IsTalkerAlias returns true if this is a talker alias frame
func (d *DMRData) IsTalkerAlias() bool {
	return d.FLCO >= FLCO_TALKER_ALIAS_HEADER && d.FLCO <= FLCO_TALKER_ALIAS_BLOCK3
}

// IsGPSInfo returns true if this is a GPS info frame
func (d *DMRData) IsGPSInfo() bool {
	return d.FLCO == FLCO_GPS_INFO
}

// Reset clears all fields to their default values
func (d *DMRData) Reset() {
	d.SlotNo = 0
	d.SrcId = 0
	d.DstId = 0
	d.FLCO = 0
	d.DataType = 0
	d.N = 0
	d.SeqNo = 0
	for i := range d.Data {
		d.Data[i] = 0
	}
	d.BER = 0
	d.RSSI = 0
	d.StreamId = 0
	d.Missing = false
}

// Copy creates a deep copy of the DMRData
func (d *DMRData) Copy() *DMRData {
	newData := &DMRData{
		SlotNo:   d.SlotNo,
		SrcId:    d.SrcId,
		DstId:    d.DstId,
		FLCO:     d.FLCO,
		DataType: d.DataType,
		N:        d.N,
		SeqNo:    d.SeqNo,
		BER:      d.BER,
		RSSI:     d.RSSI,
		StreamId: d.StreamId,
		Missing:  d.Missing,
	}
	copy(newData.Data[:], d.Data[:])
	return newData
}

// String returns a string representation for debugging
func (d *DMRData) String() string {
	return fmt.Sprintf("DMRData[Slot:%d, Src:%d, Dst:%d, FLCO:0x%02X, DT:0x%02X, N:%d, Seq:%d, Stream:0x%08X]",
		d.SlotNo, d.SrcId, d.DstId, d.FLCO, d.DataType, d.N, d.SeqNo, d.StreamId)
}

// GetFLCOString returns a human-readable FLCO type
func (d *DMRData) GetFLCOString() string {
	switch d.FLCO {
	case FLCO_GROUP:
		return "GROUP"
	case FLCO_USER_USER:
		return "PRIVATE"
	case FLCO_TALKER_ALIAS_HEADER:
		return "TA_HEADER"
	case FLCO_TALKER_ALIAS_BLOCK1:
		return "TA_BLOCK1"
	case FLCO_TALKER_ALIAS_BLOCK2:
		return "TA_BLOCK2"
	case FLCO_TALKER_ALIAS_BLOCK3:
		return "TA_BLOCK3"
	case FLCO_GPS_INFO:
		return "GPS"
	default:
		return fmt.Sprintf("UNKNOWN(0x%02X)", d.FLCO)
	}
}

// GetDataTypeString returns a human-readable data type
func (d *DMRData) GetDataTypeString() string {
	switch d.DataType {
	case DT_VOICE:
		return "VOICE"
	case DT_VOICE_SYNC:
		return "VOICE_SYNC"
	case DT_VOICE_LC_HEADER:
		return "VOICE_LC_HEADER"
	case DT_TERMINATOR_WITH_LC:
		return "TERMINATOR"
	case DT_DATA_HEADER:
		return "DATA_HEADER"
	case DT_RATE_12_DATA:
		return "RATE_12_DATA"
	case DT_RATE_34_DATA:
		return "RATE_34_DATA"
	case DT_RATE_1_DATA:
		return "RATE_1_DATA"
	case DT_IDLE:
		return "IDLE"
	default:
		return fmt.Sprintf("UNKNOWN(0x%02X)", d.DataType)
	}
}
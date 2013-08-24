package smpp34

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strconv"
)

type Pdu interface {
	Fields() map[int]Field
	MandatoryFieldsList() []string
	GetField(string) (Field, error)
	GetHeader() *Header
	TLVFields() []*TLVField
	Writer() []byte
}

func ParsePdu(data []byte) (Pdu, error) {
	if len(data) < 16 {
		return nil, errors.New("Invalid PDU. Length under 16 bytes")
	}

	header := ParsePduHeader(data[:16])

	switch header.Id {
	case SUBMIT_SM:
		n, err := NewSubmitSm(header, data[16:])
		return Pdu(n), err
	case SUBMIT_SM_RESP:
		n, err := NewSubmitSmResp(header, data[16:])
		return Pdu(n), err
	case DELIVER_SM:
		n, err := NewDeliverSm(header, data[16:])
		return Pdu(n), err
	case DELIVER_SM_RESP:
		n, err := NewDeliverSmResp(header, data[16:])
		return Pdu(n), err
	case BIND_TRANSCEIVER:
		n, err := NewBind(header, data[16:])
		return Pdu(n), err
	case BIND_TRANSCEIVER_RESP:
		n, err := NewBindResp(header, data[16:])
		return Pdu(n), err
	case ENQUIRE_LINK:
		n, err := NewEnquireLink(header)
		return Pdu(n), err
	case ENQUIRE_LINK_RESP:
		n, err := NewEnquireLinkResp(header)
		return Pdu(n), err
	default:
		return nil, errors.New("Unknown PDU Command ID: " + strconv.Itoa(int(header.Id)))
	}
}

func ParsePduHeader(data []byte) *Header {
	return NewPduHeader(
		unpackUi32(data[:4]),
		unpackUi32(data[4:8]),
		unpackUi32(data[8:12]),
		unpackUi32(data[12:16]),
	)
}

func create_pdu_fields(fieldNames []string, r *bytes.Buffer) (map[int]Field, []*TLVField, error) {

	fields := make(map[int]Field)
	eof := false
	for i, k := range fieldNames {
		switch k {
		case SERVICE_TYPE, SOURCE_ADDR, DESTINATION_ADDR, SCHEDULE_DELIVERY_TIME, VALIDITY_PERIOD, SYSTEM_ID, PASSWORD, SYSTEM_TYPE, ADDRESS_RANGE, MESSAGE_ID:
			t, err := r.ReadBytes(0x00)

			if err == io.EOF {
				eof = true
			} else if err != nil {
				return nil, nil, err
			}

			fields[i] = NewVariableField(t)
		case SOURCE_ADDR_TON, SOURCE_ADDR_NPI, DEST_ADDR_TON, DEST_ADDR_NPI, ESM_CLASS, PROTOCOL_ID, PRIORITY_FLAG, REGISTERED_DELIVERY, REPLACE_IF_PRESENT_FLAG, DATA_CODING, SM_DEFAULT_MSG_ID, INTERFACE_VERSION, ADDR_TON, ADDR_NPI:
			t, err := r.ReadByte()

			if err == io.EOF {
				eof = true
			} else if err != nil {
				return nil, nil, err
			}

			fields[i] = NewFixedField(t)
		case SM_LENGTH:
			// Short Message Length
			t, err := r.ReadByte()

			if err == io.EOF {
				eof = true
			} else if err != nil {
				return nil, nil, err
			}

			fields[i] = NewFixedField(t)

			// Short Message
			p := make([]byte, t)

			_, err = r.Read(p)
			if err == io.EOF {
				eof = true
			} else if err != nil {
				return nil, nil, err
			}

			fields[i+1] = NewVariableField(p)
		case SHORT_MESSAGE:
			continue
		}
	}

	// Optional Fields
	tlvs := []*TLVField{}
	var err error

	if !eof {
		tlvs, err = parse_tlv_fields(r)

		if err != nil {
			return nil, nil, err
		}
	}

	return fields, tlvs, nil
}

func parse_tlv_fields(r *bytes.Buffer) ([]*TLVField, error) {
	tlvs := make([]*TLVField, 0)

	for {
		p := make([]byte, 4)
		_, err := r.Read(p)

		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		// length
		l := unpackUi16(p[2:4])

		// Get Value
		v := make([]byte, l)

		_, err = r.Read(v)
		if err != nil {
			return nil, err
		}

		tlvs = append(tlvs, &TLVField{
			unpackUi16(p[0:2]),
			unpackUi16(p[2:4]),
			v,
		})
	}

	return tlvs, nil
}

func unpackUi32(b []byte) (n uint32) {
	n = binary.BigEndian.Uint32(b)
	return
}

func packUi32(n uint32) (b []byte) {
	b = make([]byte, 4)
	binary.BigEndian.PutUint32(b, n)
	return
}

func unpackUi16(b []byte) (n uint16) {
	n = binary.BigEndian.Uint16(b)
	return
}

func packUi16(n uint16) (b []byte) {
	b = make([]byte, 2)
	binary.BigEndian.PutUint16(b, n)
	return
}

func packUi8(n uint8) (b []byte) {
	b = make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(n))
	return b[1:]
}

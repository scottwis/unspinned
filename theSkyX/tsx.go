package theSkyX

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/scottwis/unspinned/readers"
	"io"
	"math"
	"net"
	"strconv"
)

var getStateScript =`
/* Java Script */
/* Socket Start Packet */
var ret = {}
sky6StarChart.DocumentProperty(1);
ret.Longitude = sky6StarChart.DocPropOut;

sky6StarChart.DocumentProperty(0);
ret.Latitude = sky6StarChart.DocPropOut;

ret.RotatorAngle = ccdsoftCamera.rotatorPositionAngle();

sky6RASCOMTele.GetAzAlt();
ret.PointingAt = {
	Alt: sky6RASCOMTele.dAlt,
    Az: sky6RASCOMTele.dAz
}

JSON.stringify(ret)
/* Socket End Packet */
`

var rotateScript =`
/* Java Script */
/* Socket Start Packet */
ccdsoftCamera.rotatorGotoPositionAngle(%v);
var ret = {}
sky6StarChart.DocumentProperty(1);
ret.Longitude = sky6StarChart.DocPropOut;

sky6StarChart.DocumentProperty(0);
ret.Latitude = sky6StarChart.DocPropOut;

ret.RotatorAngle = ccdsoftCamera.rotatorPositionAngle();

sky6RASCOMTele.GetAzAlt();
ret.PointingAt = {
	Alt: sky6RASCOMTele.dAlt,
    Az: sky6RASCOMTele.dAz
}

JSON.stringify(ret)
/* Socket End Packet */
`

type Degrees float64
type Radians float64

func (x Degrees) ToRadians() Radians {
	return Radians(x * math.Pi/180.0)
}

func (x Radians) ToDegrees() Degrees {
	return Degrees(x * 180 / math.Pi)
}

type State struct {
	Longitude Degrees
	Latitude Degrees
	RotatorAngle Degrees
	PointingAt AltAz
}

type AltAz struct {
	Alt Degrees
	Az Degrees
}

type TSX interface {
	io.Closer
	GetState() (State, error)
	Rotate(angle Degrees) (State, error)
}

type tsx struct {
	connection net.Conn
}

func New(host, port string) (TSX, error) {
	cnn, err := net.Dial("tcp", fmt.Sprintf("%v:%v", host, port))

	if err != nil {
		return nil, err
	}

	ret := &tsx{
		connection: cnn,
	}

	return ret, nil
}

func (t *tsx) Close() error {
	return t.connection.Close()
}

func consumeNumber(reader *bufio.Reader) ([]rune, bool, error) {
	var buf []rune

	for {
		r, _, err := reader.ReadRune()

		if err != nil {
			return buf, len(buf) > 0, err
		}

		if r >= '0' && r <= '9' {
			buf = append(buf, r)
		} else {
			reader.UnreadRune()
			return buf, len(buf) > 0, nil
		}
	}
}

func consumeSequence(reader *bufio.Reader, seq string) ([]rune, bool, error) {
	var buf []rune
	for _, expected := range []rune(seq) {
		r, _, err := reader.ReadRune()

		if err != nil {
			return buf, false, err
		}

		if r != expected {
			reader.UnreadRune()
			return buf, false, nil
		}

		buf = append(buf, r)
	}
	return buf, true, nil
}

func consumeError(reader *bufio.Reader) error {
	var buf []rune

	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			return errors.New(string(buf))
		}
		if r == ' ' {
			tmpBuf, ok, err := consumeSequence(reader, "Error = ")

			if ! ok {
				buf = append(buf, r)
				buf = append(buf, tmpBuf...)
				continue
			}

			if err != nil {
				return errors.New(string(buf))
			}

			tmpBuf, ok, err = consumeNumber(reader)

			if ! ok {
				buf = append(buf, tmpBuf...)
				continue
			}

			if err != nil {
				return errors.New(string(buf))
			}

			r, _, err = reader.ReadRune()

			if err != nil {
				return errors.New(string(buf))
			}

			if r == '.' {
				num, _ := strconv.ParseInt(string(tmpBuf), 10, 64)

				return TsxError {
					Message: string(buf),
					ErrorNumber: num,
				}
			}
		} else {
			buf = append(buf, r)
		}
	}
}

func consumeTail(reader *bufio.Reader) {
	for {
		c, _, err := reader.ReadRune()
		if err == io.EOF || c == '|' {
			return
		}
	}

	_ = consumeError(reader)
}

func (t * tsx) processResponse(marshalTo interface{}) error {
	reader := bufio.NewReader(t.connection)

	r, _, err := reader.ReadRune()
	reader.UnreadRune()

	if err != nil {
		return err
	}

	if r == '{' {
		var decoder = json.NewDecoder(reader)
		err = decoder.Decode(marshalTo)
		consumeTail(bufio.NewReader(readers.Union(decoder.Buffered(), reader)))
	} else {
		err = consumeError(reader)
		consumeTail(reader)
	}

	return err
}

func (t *tsx) GetState() (State, error) {
	data := []byte(getStateScript)
	n, err := t.connection.Write(data)

	if err != nil {
		return State{}, err
	}

	if n != len(data) {
		err := errors.New("incomplete write")
		return State{}, err
	}

	var ret State
	err = t.processResponse(&ret)

	if err != nil {
		ret = State{}
	}
	return ret, err
}

func (t *tsx) Rotate(angle Degrees) (State, error) {
	data := []byte(fmt.Sprintf(rotateScript, angle))
	
	n, err := t.connection.Write(data)
	if err != nil {
		return State{}, err
	}
	
	if n != len(data) {
		err := errors.New("incomplete write")
		return State{}, err
	}
	
	var ret State
	err = t.processResponse(&ret)
	
	if err != nil {
		ret = State{}
	}
	
	return ret, err
}

type TsxError struct {
	Message string
	ErrorNumber int64
}

func (e TsxError) Error() string {
	return e.Message
}
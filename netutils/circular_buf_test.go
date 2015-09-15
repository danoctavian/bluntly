package netutils_test

import (
  "testing"
  "bytes"
  "github.com/danoctavian/bluntly/netutils"
  "fmt"
)

func TestWriteReadCircularBuf(t *testing.T) {
  capacity := 10
  circ := netutils.NewCircularBuf(capacity)

  str1 := []byte("wtf")
  str2 := []byte("omg")

  var err error
  err = nil

  _, err = circ.Write(str1)
    if err != nil { 
    t.Errorf("failed write %s", err)
    return
  }
  _, err = circ.Write(str2)

  if err != nil { 
    t.Errorf("failed write %s", err)
    return
  }

  readBuf := make([]byte, 4)
  byteCount, err := circ.Read(readBuf)

  if (byteCount != 4 || !bytes.Equal(readBuf, []byte("wtfo"))) {
    t.Errorf("read failed")
    return
  }

  str3 := []byte("jeeesus")

  _, err = circ.Write(str3)
  if err != nil { 
    t.Errorf("failed write %s", err)
    return
  }

  readBuf = make([]byte, 8)
  byteCount, err = circ.Read(readBuf)

  if (byteCount != 8 || !bytes.Equal(readBuf, []byte("mgjeeesu"))) {
    t.Errorf("read failed")
    return
  }


  if (circ.Size() != 1) {
    t.Errorf("size is wrong")
    return
  }

  readBuf = make([]byte, 2)
  byteCount, err = circ.Read(readBuf)

  fmt.Printf("%i %s", byteCount, string(readBuf))

  if (byteCount != 1 || !bytes.Equal(readBuf, append([]byte("s"), 0))) {
    t.Errorf("read failed")
    return
  }
}
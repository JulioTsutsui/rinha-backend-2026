package main

import (
	"net"
	"sync"
)

const (
	maxReq  = 8192
	readBuf = 4096
)

var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, readBuf)
		return &b
	},
}

var scratchPool sync.Pool

func serve(ln net.Listener, ix *Index) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go handleConn(c, ix)
	}
}

func handleConn(c net.Conn, ix *Index) {
	defer c.Close()
	bp := bufPool.Get().(*[]byte)
	buf := *bp
	defer func() {
		if cap(buf) <= maxReq {
			*bp = buf[:cap(buf)]
			bufPool.Put(bp)
		}
	}()

	used := 0
	pos := 0
	for {
		var headEnd int
		for {
			if idx := findHeadEnd(buf[pos:used]); idx >= 0 {
				headEnd = pos + idx + 4
				break
			}
			if used == len(buf) {
				if pos > 0 {
					copy(buf, buf[pos:used])
					used -= pos
					pos = 0
				} else if used >= maxReq {
					return
				} else {
					nb := make([]byte, len(buf)*2)
					copy(nb, buf[:used])
					buf = nb
				}
			}
			n, err := c.Read(buf[used:])
			if n > 0 {
				used += n
				continue
			}
			if err != nil {
				return
			}
		}

		isPost, contentLen := parseReqLine(buf[pos:headEnd])
		if contentLen > maxReq-(headEnd-pos) {
			return
		}
		bodyEnd := headEnd + contentLen
		for used < bodyEnd {
			if used == len(buf) {
				if pos > 0 {
					copy(buf, buf[pos:used])
					used -= pos
					headEnd -= pos
					bodyEnd -= pos
					pos = 0
				} else {
					nb := make([]byte, len(buf)*2)
					copy(nb, buf[:used])
					buf = nb
				}
			}
			n, err := c.Read(buf[used:])
			if n > 0 {
				used += n
				continue
			}
			if err != nil {
				return
			}
		}

		var resp []byte
		if isPost {
			resp = handleScore(buf[headEnd:bodyEnd], ix)
		} else {
			resp = readyResp
		}
		if _, err := c.Write(resp); err != nil {
			return
		}

		pos = bodyEnd
		if pos == used {
			pos = 0
			used = 0
		}
	}
}

func handleScore(body []byte, ix *Index) []byte {
	var qi [Dims]int16
	if !vectorize(body, &qi) {
		return fraudResp[0]
	}
	var qf [Dims]float32
	for d := 0; d < Dims; d++ {
		qf[d] = float32(qi[d])
	}
	sc := scratchPool.Get().(*scratch)
	cnt := ix.fraudCount(&qf, &qi, sc)
	scratchPool.Put(sc)
	if cnt > 5 {
		cnt = 5
	}
	return fraudResp[cnt]
}

func findHeadEnd(b []byte) int {
	for i := 0; i+3 < len(b); i++ {
		if b[i] == '\r' && b[i+1] == '\n' && b[i+2] == '\r' && b[i+3] == '\n' {
			return i
		}
	}
	return -1
}

func parseReqLine(buf []byte) (isPost bool, contentLen int) {
	isPost = len(buf) >= 4 && buf[0] == 'P' && buf[1] == 'O' && buf[2] == 'S' && buf[3] == 'T'
	contentLen = findContentLength(buf)
	return
}

func findContentLength(buf []byte) int {
	const name = "content-length:"
	for i := 0; i+len(name) < len(buf); i++ {
		if (buf[i] == 'C' || buf[i] == 'c') && hasPrefixFold(buf[i:], name) {
			j := i + len(name)
			for j < len(buf) && (buf[j] == ' ' || buf[j] == '\t') {
				j++
			}
			n := 0
			for j < len(buf) && buf[j] >= '0' && buf[j] <= '9' {
				n = n*10 + int(buf[j]-'0')
				j++
			}
			return n
		}
	}
	return 0
}

func hasPrefixFold(b []byte, name string) bool {
	if len(b) < len(name) {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := b[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != name[i] {
			return false
		}
	}
	return true
}

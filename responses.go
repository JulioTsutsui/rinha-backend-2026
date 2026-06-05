package main

var fraudResp [6][]byte

var readyResp = []byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")

func init() {
	bodies := [6]string{
		`{"approved":true,"fraud_score":0}`,
		`{"approved":true,"fraud_score":0.2}`,
		`{"approved":true,"fraud_score":0.4}`,
		`{"approved":false,"fraud_score":0.6}`,
		`{"approved":false,"fraud_score":0.8}`,
		`{"approved":false,"fraud_score":1}`,
	}
	for i, b := range bodies {
		fraudResp[i] = buildResp(b)
	}
}

func buildResp(body string) []byte {
	const head = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: "
	clen := itoa(len(body))
	out := make([]byte, 0, len(head)+len(clen)+4+len(body))
	out = append(out, head...)
	out = append(out, clen...)
	out = append(out, "\r\n\r\n"...)
	out = append(out, body...)
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

package tls

import "crypto/tls"

type TlsContext func() *tls.Config

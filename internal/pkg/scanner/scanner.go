package scanner

import "go.lumeweb.com/portal/core"

type CoreStorageProtocolComposite interface {
	core.Protocol
	core.StorageProtocol
}

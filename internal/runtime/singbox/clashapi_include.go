package singbox

import (
	"github.com/sagernet/sing-box/experimental"
	clashapi "github.com/sagernet/sing-box/experimental/clashapi"
)

func init() {
	experimental.RegisterClashServerConstructor(clashapi.NewServer)
}

vim.lsp.config('*', {
	root_markers = { '.git' },
})

-- Enable exactly the capability-gated set (see core/servers.lua). Servers
-- whose toolchain/binary is absent are never enabled, so minimal hosts
-- produce zero LSP spawn-error noise.
vim.lsp.enable(require('core/servers').enabled_servers())

-- Log file highlighting. No LSP and no generic treesitter grammar exist
-- for logs, so this is syntax-only (timestamps, levels, IPs, etc.).
require("log-highlight").setup({})

-- The plugin's ftdetect catches *.log and *_log; add rotated logs
-- (e.g. app.log.1) which sysadmins hit constantly.
vim.filetype.add({
	pattern = {
		[".*%.log%.%d+"] = "log",
	},
})

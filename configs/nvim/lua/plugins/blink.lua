require("blink.cmp").setup({
	keymap = { preset = "enter" },
	fuzzy = { implementation = "prefer_rust" },
	sources = {
		default = { "lsp", "path", "snippets", "buffer" },
	},
})

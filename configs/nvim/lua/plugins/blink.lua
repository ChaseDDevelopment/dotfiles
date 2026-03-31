require("blink.cmp").setup({
	keymap = { preset = "enter" },
	fuzzy = { implementation = "prefer_rust" },
	signature = { enabled = true },
	sources = {
		default = { "lazydev", "lsp", "path", "snippets", "buffer" },
		providers = {
			lazydev = {
				name = "LazyDev",
				module = "lazydev.integrations.blink",
				score_offset = 100,
			},
		},
	},
})

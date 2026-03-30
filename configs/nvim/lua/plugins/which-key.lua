require("which-key").setup({
	preset = "helix",
})

require("which-key").add({
	{ "<leader>b", group = "buffer" },
	{ "<leader>c", group = "code" },
	{ "<leader>f", group = "find" },
	{ "<leader>g", group = "git" },
	{ "<leader>q", group = "quit" },
	{ "<leader>u", group = "toggle" },
	{ "<leader>w", group = "window" },
	{ "<leader>x", group = "diagnostics" },
})

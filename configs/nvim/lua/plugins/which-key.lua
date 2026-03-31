require("which-key").setup({
	preset = "helix",
})

require("which-key").add({
	-- Leader groups
	{ "<leader>b", group = "buffer", icon = "󰈔" },
	{ "<leader>c", group = "code", icon = "󰅩" },
	{ "<leader>f", group = "find", icon = "󰍉" },
	{ "<leader>g", group = "git", icon = "󰊢" },
	{ "<leader>h", group = "harpoon", icon = "󰀱" },
	{ "<leader>q", group = "quit", icon = "󰈆" },
	{ "<leader>u", group = "toggle", icon = "󰔡" },
	{ "<leader>w", group = "window", icon = "󰖲" },
	{ "<leader>x", group = "diagnostics", icon = "󰒡" },

	-- Leader actions
	{ "<leader>/", icon = "󰍉", desc = "Grep" },
	{ "<leader>o", icon = "󰙅", desc = "Code outline" },
	{ "<leader>-", icon = "󰇘", desc = "Split below" },
	{ "<leader>|", icon = "󰇙", desc = "Split right" },
	{ "<leader>1", icon = "󰎤", desc = "Harpoon 1" },
	{ "<leader>2", icon = "󰎧", desc = "Harpoon 2" },
	{ "<leader>3", icon = "󰎪", desc = "Harpoon 3" },
	{ "<leader>4", icon = "󰎭", desc = "Harpoon 4" },

	-- g-prefix entries
	{ "gd", icon = "󰊕", desc = "Go to definition" },
	{ "ge", icon = "󰬝", desc = "Prev end of word" },
	{ "gg", icon = "󰞕", desc = "First line" },
	{ "gi", icon = "󰏫", desc = "Go to last insert" },
	{ "gO", icon = "󰙅", desc = "Document symbols" },
	{ "gu", icon = "󰬵", desc = "Lowercase" },
	{ "gU", icon = "󰬶", desc = "Uppercase" },
	{ "gv", icon = "󰒅", desc = "Last visual selection" },
	{ "g%", icon = "󰑙", desc = "Cycle backwards" },
	{ "g,", icon = "󰜴", desc = "Newer change position" },
	{ "g;", icon = "󰜱", desc = "Older change position" },
	{ "gr", icon = "󰌹", group = "References" },
})

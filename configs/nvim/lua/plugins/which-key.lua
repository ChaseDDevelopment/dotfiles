require("which-key").setup({
	preset = "helix",
})

require("which-key").add({
	-- Leader groups
	{ "<leader>a", group = "claude", icon = { icon = "󰚩", color = "purple" } },
	{ "<leader>b", group = "buffer", icon = { icon = "󰈔", color = "azure" } },
	{ "<leader>c", group = "code", icon = { icon = "󰅩", color = "purple" } },
	{ "<leader>f", group = "find", icon = { icon = "󰍉", color = "cyan" } },
	{ "<leader>g", group = "git", icon = { icon = "󰊢", color = "orange" } },
	{ "<leader>h", group = "harpoon", icon = { icon = "󰀱", color = "green" } },
	{ "<leader>q", group = "quit/session", icon = { icon = "󰈆", color = "red" } },
	{ "<leader>u", group = "toggle", icon = { icon = "󰔡", color = "yellow" } },
	{ "<leader>w", group = "window", icon = { icon = "󰖲", color = "azure" } },
	{ "<leader>x", group = "diagnostics", icon = { icon = "󰒡", color = "red" } },
	{ "s", group = "surround", icon = { icon = "󰅪", color = "purple" } },

	-- Find actions (snacks picker)
	{ "<leader>e", icon = { icon = "󱂬", color = "cyan" }, desc = "File explorer" },
	{ "<leader>ff", icon = { icon = "󰈞", color = "cyan" }, desc = "Find files" },
	{ "<leader>fg", icon = { icon = "󰈬", color = "cyan" }, desc = "Grep" },
	{ "<leader>fb", icon = { icon = "󰘷", color = "cyan" }, desc = "Buffers" },
	{ "<leader>fr", icon = { icon = "󰋚", color = "cyan" }, desc = "Recent files" },
	{ "<leader>fs", icon = { icon = "󰛡", color = "cyan" }, desc = "Smart find" },
	{ "<leader>fn", icon = { icon = "󰝒", color = "green" }, desc = "New file" },
	{ "<leader>/", icon = { icon = "󰍉", color = "cyan" }, desc = "Grep" },

	-- Code actions (LSP)
	{ "<leader>cd", icon = { icon = "󰒡", color = "yellow" }, desc = "Line diagnostics" },
	{ "<leader>cr", icon = { icon = "󰑕", color = "purple" }, desc = "Rename symbol" },
	{ "<leader>ca", icon = { icon = "󰁨", color = "purple" }, desc = "Code action" },
	{ "<leader>cs", icon = { icon = "󰏪", color = "purple" }, desc = "Signature help" },

	-- Git actions (gitsigns + diffview)
	{ "<leader>gs", icon = { icon = "󰊢", color = "green" }, desc = "Stage hunk" },
	{ "<leader>gr", icon = { icon = "󰝳", color = "red" }, desc = "Reset hunk" },
	{ "<leader>gu", icon = { icon = "󰕌", color = "yellow" }, desc = "Undo stage hunk" },
	{ "<leader>gp", icon = { icon = "󰖷", color = "azure" }, desc = "Preview hunk" },
	{ "<leader>gb", icon = { icon = "󰆽", color = "orange" }, desc = "Blame line" },
	{ "<leader>gB", icon = { icon = "󰆽", color = "yellow" }, desc = "Toggle line blame" },
	{ "<leader>gd", icon = { icon = "󰦓", color = "orange" }, desc = "Diff view" },
	{ "<leader>gD", icon = { icon = "󰔻", color = "orange" }, desc = "File history" },
	{ "<leader>gq", icon = { icon = "󰅗", color = "red" }, desc = "Close diff view" },

	-- Diagnostics (trouble.nvim)
	{ "<leader>xx", icon = { icon = "󰒡", color = "red" }, desc = "Diagnostics" },
	{ "<leader>xX", icon = { icon = "󰒡", color = "yellow" }, desc = "Buffer diagnostics" },
	{ "<leader>xq", icon = { icon = "󰝮", color = "azure" }, desc = "Quickfix list" },
	{ "<leader>xl", icon = { icon = "󰝮", color = "azure" }, desc = "Location list" },
	{ "<leader>xt", icon = { icon = "󰗡", color = "green" }, desc = "Todo list" },

	-- Buffer actions
	{ "<leader>bd", icon = { icon = "󰅗", color = "red" }, desc = "Delete buffer" },

	-- Window actions
	{ "<leader>wd", icon = { icon = "󰅗", color = "red" }, desc = "Close window" },
	{ "<leader>-", icon = { icon = "󰇘", color = "azure" }, desc = "Split below" },
	{ "<leader>|", icon = { icon = "󰇙", color = "azure" }, desc = "Split right" },

	-- Quit/session actions
	{ "<leader>qq", icon = { icon = "󰗼", color = "red" }, desc = "Quit all" },
	{ "<leader>qs", icon = { icon = "󰁯", color = "green" }, desc = "Restore session" },
	{ "<leader>qS", icon = { icon = "󰮝", color = "yellow" }, desc = "Select session" },
	{ "<leader>qd", icon = { icon = "󰅖", color = "red" }, desc = "Don't save session" },

	-- Code outline
	{ "<leader>o", icon = { icon = "󰙅", color = "purple" }, desc = "Code outline" },

	-- Harpoon
	{ "<leader>ha", icon = { icon = "󰐕", color = "green" }, desc = "Add file" },
	{ "<leader>hh", icon = { icon = "󰍜", color = "green" }, desc = "Harpoon menu" },
	{ "<leader>1", icon = { icon = "󰎤", color = "green" }, desc = "Harpoon 1" },
	{ "<leader>2", icon = { icon = "󰎧", color = "green" }, desc = "Harpoon 2" },
	{ "<leader>3", icon = { icon = "󰎪", color = "green" }, desc = "Harpoon 3" },
	{ "<leader>4", icon = { icon = "󰎭", color = "green" }, desc = "Harpoon 4" },

	-- g-prefix motions and LSP
	{ "gd", icon = { icon = "󰊕", color = "purple" }, desc = "Go to definition" },
	{ "gr", icon = { icon = "󰌹", color = "purple" }, desc = "References" },
	{ "ge", icon = { icon = "󰬝", color = "cyan" }, desc = "Prev end of word" },
	{ "gg", icon = { icon = "󰞕", color = "cyan" }, desc = "First line" },
	{ "gi", icon = { icon = "󰏫", color = "yellow" }, desc = "Go to last insert" },
	{ "gO", icon = { icon = "󰙅", color = "purple" }, desc = "Document symbols" },
	{ "gu", icon = { icon = "󰬵", color = "azure" }, desc = "Lowercase" },
	{ "gU", icon = { icon = "󰬶", color = "azure" }, desc = "Uppercase" },
	{ "gv", icon = { icon = "󰒅", color = "yellow" }, desc = "Last visual selection" },
	{ "g%", icon = { icon = "󰑙", color = "cyan" }, desc = "Cycle backwards" },
	{ "g,", icon = { icon = "󰜴", color = "green" }, desc = "Newer change position" },
	{ "g;", icon = { icon = "󰜱", color = "orange" }, desc = "Older change position" },
	{ "K", icon = { icon = "󰋗", color = "purple" }, desc = "Hover docs" },

	-- Bracket navigation
	{ "]c", icon = { icon = "󰊢", color = "green" }, desc = "Next hunk" },
	{ "[c", icon = { icon = "󰊢", color = "orange" }, desc = "Prev hunk" },
	{ "]d", icon = { icon = "󰒡", color = "red" }, desc = "Next diagnostic" },
	{ "[d", icon = { icon = "󰒡", color = "red" }, desc = "Prev diagnostic" },
	{ "]q", icon = { icon = "󰝮", color = "azure" }, desc = "Next quickfix" },
	{ "[q", icon = { icon = "󰝮", color = "azure" }, desc = "Prev quickfix" },

	-- Claude Code
	{ "<leader>ac", icon = { icon = "󰚩", color = "purple" }, desc = "Toggle Claude" },
	{ "<leader>af", icon = { icon = "󰆤", color = "purple" }, desc = "Focus Claude" },
	{ "<leader>ar", icon = { icon = "󰑓", color = "purple" }, desc = "Resume Claude" },
	{ "<leader>aC", icon = { icon = "󰞇", color = "purple" }, desc = "Continue Claude" },
	{ "<leader>as", icon = { icon = "󰒊", color = "purple" }, desc = "Send to Claude" },
	{ "<leader>ab", icon = { icon = "󰈔", color = "purple" }, desc = "Add buffer to Claude" },
	{ "<leader>aa", icon = { icon = "󰄬", color = "green" }, desc = "Accept diff" },
	{ "<leader>ad", icon = { icon = "󰅖", color = "red" }, desc = "Deny diff" },

	-- Toggle actions
	{ "<leader>uH", icon = { icon = "󰀱", color = "yellow" }, desc = "Toggle hardtime" },
	{ "<leader>uP", icon = { icon = "󰛡", color = "yellow" }, desc = "Toggle precognition" },
})

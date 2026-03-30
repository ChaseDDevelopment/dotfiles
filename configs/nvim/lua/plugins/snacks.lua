require("snacks").setup({
	picker = { enabled = true },
	toggle = { enabled = true },
	dashboard = {
		enabled = true,
		preset = {
			header = [[
 █████╗ ███╗   ██╗██████╗ ██████╗  ██████╗ ███╗   ███╗███████╗██████╗  █████╗
██╔══██╗████╗  ██║██╔══██╗██╔══██╗██╔═══██╗████╗ ████║██╔════╝██╔══██╗██╔══██╗
███████║██╔██╗ ██║██║  ██║██████╔╝██║   ██║██╔████╔██║█████╗  ██║  ██║███████║
██╔══██║██║╚██╗██║██║  ██║██╔══██╗██║   ██║██║╚██╔╝██║██╔══╝  ██║  ██║██╔══██║
██║  ██║██║ ╚████║██████╔╝██║  ██║╚██████╔╝██║ ╚═╝ ██║███████╗██████╔╝██║  ██║
╚═╝  ╚═╝╚═╝  ╚═══╝╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═╝     ╚═╝╚══════╝╚═════╝ ╚═╝  ╚═╝
███╗   ██╗███████╗██╗  ██╗██╗   ██╗███████╗
████╗  ██║██╔════╝╚██╗██╔╝██║   ██║██╔════╝
██╔██╗ ██║█████╗   ╚███╔╝ ██║   ██║███████╗
██║╚██╗██║██╔══╝   ██╔██╗ ██║   ██║╚════██║
██║ ╚████║███████╗██╔╝ ██╗╚██████╔╝███████║
╚═╝  ╚═══╝╚══════╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝

┈┈┈┈┈┈┈┈┈┈┈ drift through the mocha nebula ┈┈┈┈┈┈┈┈┈┈┈]],
			keys = {
				{ icon = "\u{f15b} ", key = "f", desc = "Find File", action = ":lua Snacks.dashboard.pick('files')" },
				{ icon = "\u{f016} ", key = "n", desc = "New File", action = ":ene | startinsert" },
				{ icon = "\u{f002} ", key = "g", desc = "Find Text", action = ":lua Snacks.dashboard.pick('live_grep')" },
				{ icon = "\u{f017} ", key = "r", desc = "Recent Files", action = ":lua Snacks.dashboard.pick('oldfiles')" },
				{ icon = "\u{f013} ", key = "c", desc = "Config", action = ":lua Snacks.dashboard.pick('files', {cwd = vim.fn.stdpath('config')})" },
				{ icon = "\u{f1b2} ", key = "m", desc = "Mason", action = ":Mason" },
				{ icon = "\u{f011} ", key = "q", desc = "Quit", action = ":qa" },
			},
		},
		sections = {
			{ section = "header", padding = 1 },
			{
				title = " Projects",
				section = "projects",
				indent = 2,
				gap = 0,
				padding = 1,
				action = function(dir)
					vim.fn.chdir(dir)
					vim.cmd("enew")
					Snacks.picker.explorer({ cwd = dir })
				end,
			},
			{ title = " Quick Actions", section = "keys", indent = 2, gap = 0, padding = 1 },
			{ title = " Recent Files", section = "recent_files", indent = 2, gap = 0, padding = 1 },
		},
	},
})
-- File explorer
vim.keymap.set("n", "<leader>e", function()
	Snacks.picker.explorer({ cwd = vim.fn.getcwd() })
end, { desc = "File explorer" })

-- File picker keymaps
vim.keymap.set("n", "<leader>ff", function()
	Snacks.picker.files()
end, { desc = "Find files" })
vim.keymap.set("n", "<leader>fg", function()
	Snacks.picker.grep()
end, { desc = "Grep" })
vim.keymap.set("n", "<leader>fb", function()
	Snacks.picker.buffers()
end, { desc = "Buffers" })
vim.keymap.set("n", "<leader>fr", function()
	Snacks.picker.recent()
end, { desc = "Recent files" })
vim.keymap.set("n", "<leader>fs", function()
	Snacks.picker.smart()
end, { desc = "Smart find" })

-- LSP navigation through picker
vim.keymap.set("n", "gd", function()
	Snacks.picker.lsp_definitions()
end, { desc = "Go to definition" })
vim.keymap.set("n", "gr", function()
	Snacks.picker.lsp_references()
end, { desc = "References" })

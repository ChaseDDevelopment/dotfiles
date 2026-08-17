require("yanky").setup({
	highlight = { timer = 150 },
	system_clipboard = {
		sync_with_ring = vim.env.HERDR_ENV ~= "1",
	},
})

vim.keymap.set({ "n", "x" }, "y", "<Plug>(YankyYank)", { desc = "Yank" })
vim.keymap.set({ "n", "x" }, "p", "<Plug>(YankyPutAfter)", { desc = "Put after" })
vim.keymap.set({ "n", "x" }, "P", "<Plug>(YankyPutBefore)", { desc = "Put before" })
vim.keymap.set("n", "[y", "<Plug>(YankyCycleForward)", { desc = "Cycle yank forward" })
vim.keymap.set("n", "]y", "<Plug>(YankyCycleBackward)", { desc = "Cycle yank backward" })
vim.keymap.set("n", "<leader>fy", "<cmd>YankyRingHistory<cr>", { desc = "Yank history" })

require("grug-far").setup({})

-- Project-wide find and replace (ripgrep-backed); in visual mode the
-- selection seeds the search.
vim.keymap.set({ "n", "x" }, "<leader>sr", function()
	require("grug-far").open({ transient = true })
end, { desc = "Search and replace" })

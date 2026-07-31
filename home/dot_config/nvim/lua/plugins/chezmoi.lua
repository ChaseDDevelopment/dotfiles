require("chezmoi").setup({})

-- Editing a file inside a chezmoi source tree applies it on save, so the
-- live config never drifts behind the repo. Covers both sources and the
-- hydra-style ~/dotfiles checkout; chezmoi.vim handles tmpl/dot_ syntax.
local source_globs = {
	vim.fn.expand("~/Documents/GitHub/dotfiles/home") .. "/*",
	vim.fn.expand("~/Documents/GitHub/ai-chezmoi/home") .. "/*",
	vim.fn.expand("~/dotfiles/home") .. "/*",
}
vim.api.nvim_create_autocmd({ "BufRead", "BufNewFile" }, {
	pattern = source_globs,
	callback = function(ev)
		vim.schedule(function()
			require("chezmoi.commands.__edit").watch(ev.buf)
		end)
	end,
})

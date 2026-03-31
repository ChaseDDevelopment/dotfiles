require('gitsigns').setup({
	on_attach = function(bufnr)
		local gs = require('gitsigns')
		local function map(mode, l, r, opts)
			opts = opts or {}
			opts.buffer = bufnr
			vim.keymap.set(mode, l, r, opts)
		end

		-- Hunk navigation
		map('n', ']c', function()
			if vim.wo.diff then
				vim.cmd.normal({ ']c', bang = true })
			else
				gs.nav_hunk('next')
			end
		end, { desc = 'Next hunk' })

		map('n', '[c', function()
			if vim.wo.diff then
				vim.cmd.normal({ '[c', bang = true })
			else
				gs.nav_hunk('prev')
			end
		end, { desc = 'Prev hunk' })

		-- Hunk actions
		map('n', '<leader>gs', gs.stage_hunk, { desc = 'Stage hunk' })
		map('n', '<leader>gr', gs.reset_hunk, { desc = 'Reset hunk' })
		map('v', '<leader>gs', function()
			gs.stage_hunk({ vim.fn.line('.'), vim.fn.line('v') })
		end, { desc = 'Stage hunk' })
		map('v', '<leader>gr', function()
			gs.reset_hunk({ vim.fn.line('.'), vim.fn.line('v') })
		end, { desc = 'Reset hunk' })
		map('n', '<leader>gu', gs.undo_stage_hunk, { desc = 'Undo stage hunk' })
		map('n', '<leader>gp', gs.preview_hunk, { desc = 'Preview hunk' })
		map('n', '<leader>gb', function()
			gs.blame_line({ full = true })
		end, { desc = 'Blame line' })
		map('n', '<leader>gB', gs.toggle_current_line_blame, { desc = 'Toggle line blame' })
	end,
})

local ai = require('mini.ai')
ai.setup({
	custom_textobjects = {
		f = ai.gen_spec.treesitter({ a = '@function.outer', i = '@function.inner' }),
		c = ai.gen_spec.treesitter({ a = '@class.outer', i = '@class.inner' }),
	},
})

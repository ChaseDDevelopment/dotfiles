-- Flash enhances / search with jump labels and improves f/F/t/T
-- automatically. No explicit s/S keymaps to avoid conflicting
-- with mini.surround's sa/sd/sr mappings.
require('flash').setup({
	label = {
		uppercase = false,
	},
})

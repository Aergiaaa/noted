const blockTypesList = ['text', 'heading', 'list', 'table', 'finance']

function menuTypeList(row) {
	const list = blockTypesList.filter(t => t !== row.menuType)
	if (!row.menuFilter) return list
	return list.filter(t => t.includes(row.menuFilter))
}

async function convertBlock(rowEl, t) {
	const row = Alpine.$data(rowEl)
	if (!row) return
	const ed = rowEl.querySelector('[x-data^="editor"]')
	const e = ed ? Alpine.$data(ed) : null
	if (e) {
		clearTimeout(e.saveTimer)
		e.discard = true
	}
	let content = { text: '' }
	if (t === 'table') content = { tables: [['']] }
	if (t === 'finance') content = { finance: { pocket: '' } }
	const body = { type: t, content }
	const creating = e ? e.creating : false
	const url = creating ? '/api/pages/' + row.pageId + '/blocks' : '/api/blocks/' + row.id
	const res = await fetch(url, {
		method: creating ? 'POST' : 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	})
	if (!res.ok) return
	if (e) e.creating = false
	const layoutEl = document.querySelector('[x-data="layout"]')
	if (layoutEl) Alpine.$data(layoutEl).loadContent(row.pageId)
}

function pickSlashType(rowEl, t) {
	const row = Alpine.$data(rowEl)
	row.menu = false
	convertBlock(rowEl, t)
}

function closeSlash(rowEl) {
	Alpine.$data(rowEl).menu = false
}

function tableInput(root) {
	const t = Alpine.$data(root)
	clearTimeout(t._timer)
	t._timer = setTimeout(() => tableSave(root), 400)
}

function tableTab(root) {
	const cells = [...root.querySelectorAll('td')]
	const next = cells[cells.indexOf(document.activeElement) + 1] || cells[0]
	next.focus()
}

async function tableSave(root) {
	const t = Alpine.$data(root)
	const tables = [...root.querySelectorAll('tr')].map(tr =>
		[...tr.querySelectorAll('td')].map(td => td.innerText.replace(/\n$/, '')))
	const res = await fetch('/api/blocks/' + t.id, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ type: 'table', content: { tables } })
	})
	if (!res.ok) return
}

function tableCell() {
	const td = document.createElement('td')
	td.contentEditable = 'true'
	td.spellcheck = false
	td.setAttribute('@input', 'tableInput($root)')
	td.setAttribute('@keydown.tab.prevent', 'tableTab($root)')
	td.className = 'min-w-16 border border-zinc-800 px-2 py-1 outline-none focus:bg-zinc-900'
	return td
}

async function tableAddRow(root) {
	const rows = [...root.querySelectorAll('tr')]
	const cols = rows.length ? rows[0].querySelectorAll('td').length : 1
	const tr = document.createElement('tr')
	for (let i = 0; i < cols; i++) {
		tr.appendChild(tableCell())
	}
	root.querySelector('table').appendChild(tr)
	Alpine.initTree(tr)
	await tableSave(root)
}

async function tableAddCol(root) {
	const rows = [...root.querySelectorAll('tr')]
	if (!rows.length) {
		const tr = document.createElement('tr')
		tr.appendChild(tableCell())
		root.querySelector('table').appendChild(tr)
		Alpine.initTree(tr)
	} else {
		rows.forEach(r => {
			r.appendChild(tableCell())
			Alpine.initTree(r.lastElementChild)
		})
	}
	await tableSave(root)
}

async function tableDelRow(root) {
	const rows = [...root.querySelectorAll('tr')]
	if (rows.length > 1) rows[rows.length - 1].remove()
	await tableSave(root)
}

async function tableDelCol(root) {
	const rows = [...root.querySelectorAll('tr')]
	if (rows.every(r => r.querySelectorAll('td').length > 1)) {
		rows.forEach(r => r.lastElementChild.remove())
	}
	await tableSave(root)
}

document.addEventListener('alpine:init', () => {
	let drag = null

	Alpine.data('layout', () => ({
		sidebarOpen: false,
		navbarVisible: true,
		pages: [],
		tags: [],
		totalPages: 1,
		currentPaginationPage: 1,
		activePageId: null,
		view: null,

		init() {
			window.addEventListener('keydown', e => {
				if (e.altKey && e.key === 'b') { e.preventDefault(); this.navbarVisible = !this.navbarVisible }
				if (e.altKey && e.key === 'e') { e.preventDefault(); this.sidebarOpen = !this.sidebarOpen }
			})
			this.fetchPages(1)
			this.fetchTags()
		},

		async fetchTags() {
			const res = await fetch('/api/tags')
			if (!res.ok) return
			const data = await res.json()
			this.tags = data.tags || []
		},

		async fetchPages(page) {
			const res = await fetch('/api/pages?page=' + page + '&limit=10')
			if (!res.ok) return
			const data = await res.json()
			this.pages = data.pages
			this.totalPages = data.total_pages
			this.currentPaginationPage = page
		},

		searchTimer: null,

		search() {
			clearTimeout(this.searchTimer)
			this.searchTimer = setTimeout(() => {
				const q = this.$refs.searchInput.value.trim()
				if (!q) {
					this.fetchPages(1)
					return
				}
				fetch('/api/pages?q=' + encodeURIComponent(q))
					.then(res => res.ok ? res.json() : null)
					.then(data => {
						if (!data) return
						this.pages = data.pages
						this.totalPages = 1
						this.currentPaginationPage = 1
					})
			}, 250)
		},

		clearSearch() {
			this.$refs.searchInput.value = ''
			this.fetchPages(1)
		},

		async loadFinance(view) {
			this.view = view
			this.activePageId = null
			this.sidebarOpen = false
			const res = await fetch('/fin/' + view)
			if (!res.ok) return
			const html = await res.text()
			const target = document.getElementById('main-content')
			target.innerHTML = html
			Alpine.initTree(target)
		},

		async createPage() {
			const res = await fetch('/api/pages', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ title: 'Untitled' })
			})
			if (!res.ok) return
			const data = await res.json()
			await this.fetchPages(1)
			await this.loadContent(data.id)
			this.$nextTick(() => {
				const el = document.querySelector('#main-content [x-data^="editor"]') || document.querySelector('#main-content h1')
				if (el) el.focus()
			})
		},

		async loadContent(pageId) {
			this.activePageId = pageId
			this.view = null
			this.sidebarOpen = false
			const res = await fetch('/pages/' + pageId + '/fragment')
			if (!res.ok) return
			const html = await res.text()
			const target = document.getElementById('main-content')
			target.innerHTML = html
			Alpine.initTree(target)
		}
	}))

	Alpine.data('editor', (props) => ({
		id: props.id,
		type: props.type,
		pageId: props.pageId,
		depth: props.depth,
		raw: props.raw,
		creating: !props.id,
		saveTimer: null,

		onFocus() {
			if (this.$el.innerText.replace(/\n$/, '') === this.raw) return
			if (this.type === 'list') {
				this.$el.innerHTML = '<ul class="ml-4 list-disc">' + this.raw.split('\n').map(i => '<li>' + this.escaped(i) + '</li>').join('') + '</ul>'
			} else {
				this.$el.innerHTML = this.escaped(this.raw).replace(/\n/g, '<br>')
			}
			this.placeCaret(this.$el)
		},

		async onBlur() {
			if (this.discard) return
			const text = this.currentText()
			const changed = text !== this.raw
			if (changed) {
				this.raw = text
				await this.save()
			}
			if (changed || this.raw.includes('[[')) {
				this.syncLinks()
				const layoutEl = document.querySelector('[x-data="layout"]')
				if (layoutEl && this.id) Alpine.$data(layoutEl).loadContent(this.pageId)
			}
		},

		onLinkMousedown() {},

		escaped(text) {
			return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
		},

		currentText() {
			if (this.type === 'list') {
				return [...this.$el.querySelectorAll('li')].map(li => li.innerText.replace(/\n$/, '')).join('\n').replace(/\n+$/, '')
			}
			return this.$el.innerText.replace(/\n$/, '')
		},

		onInput() {
			clearTimeout(this.saveTimer)
			this.saveTimer = setTimeout(() => this.save(), 300)
			this.trackSlash()
		},

		async save() {
			const text = this.currentText()
			if (this.creating) {
				const res = await fetch('/api/pages/' + this.pageId + '/blocks', {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ type: this.type, content: { text } })
				})
				if (!res.ok) return
				const data = await res.json()
				this.id = data.id
				this.creating = false
				this.$el.parentNode.dataset.id = this.id
				return
			}
			const res = await fetch('/api/blocks/' + this.id, {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ type: this.type, content: { text } })
			})
			if (!res.ok) return
		},

		async syncLinks() {
			const titles = [...this.raw.matchAll(/\[\[([^\[\]]+)\]\]/g)].map(m => m[1])
			const res = await fetch('/api/pages/' + this.pageId + '/wiki-links', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ titles })
			})
			if (!res.ok) return
		},

		async convert(t) {
			convertBlock(this.$el.parentNode, t)
		},

		trackSlash() {
			if (this.type !== 'text') return
			const line = this.$el.innerText.split('\n').pop()
			const rowEl = this.$el.parentNode
			if (!rowEl) return
			const row = Alpine.$data(rowEl)
			if (!row) return
			if (line.startsWith('/')) {
				row.menu = true
				row.menuFilter = line.slice(1).trim()
				row.menuIndex = 0
				row.creating = this.creating
				row.menuType = this.type
			} else {
				row.menu = false
			}
		},

		async onKeydown(e) {
			const rowEl = this.$el.parentNode
			const row = rowEl ? Alpine.$data(rowEl) : null
			if (row && row.menu) {
				const types = menuTypeList(row)
				if (e.key === 'Escape') {
					row.menu = false
					return
				}
				if (e.key === 'ArrowDown') {
					e.preventDefault()
					row.menuIndex = Math.min(row.menuIndex + 1, types.length - 1)
					return
				}
				if (e.key === 'ArrowUp') {
					e.preventDefault()
					row.menuIndex = Math.max(row.menuIndex - 1, 0)
					return
				}
				if (e.key === 'Enter') {
					e.preventDefault()
					if (types[row.menuIndex]) pickSlashType(rowEl, types[row.menuIndex])
					return
				}
			}
			if (this.type === 'list') {
				if (e.key === 'Enter' && !e.shiftKey) {
					e.preventDefault()
					await this.createBelow()
					return
				}
				if (e.key === 'Backspace' && !this.$el.innerText.trim()) {
					e.preventDefault()
					await this.deleteSelf()
				}
				return
			}
			if (e.key === 'Enter' && !e.shiftKey) {
				e.preventDefault()
				await this.createBelow()
			}
			if (e.key === 'Backspace' && !this.$el.innerText.trim()) {
				e.preventDefault()
				await this.deleteSelf()
			}
		},

		async createBelow() {
			const res = await fetch('/api/pages/' + this.pageId + '/blocks', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ type: 'text', content: { text: '' } })
			})
			if (!res.ok) return
			const data = await res.json()

			const wrapper = document.createElement('div')
			wrapper.innerHTML = this.rowHtml(data.id)
			const row = wrapper.firstElementChild
			this.$el.parentNode.parentNode.insertBefore(row, this.$el.parentNode.nextSibling)
			Alpine.initTree(row)
			const node = row.querySelector('[contenteditable]')
			node.focus()
			this.placeCaret(node)
		},

		async deleteSelf() {
			if (this.creating) return
			const row = this.$el.parentNode
			const prevRow = row.previousElementSibling
			const res = await fetch('/api/blocks/' + this.id, {
				method: 'DELETE'
			})
			if (!res.ok) return
			row.remove()
			if (prevRow) {
				const prev = prevRow.querySelector('[contenteditable]')
				prev.focus()
				this.placeCaret(prev)
			}
		},

		rowHtml(id) {
			return '<div x-data="row({id: \'' + id + '\', type: \'text\', pageId: \'' + this.pageId + '\', depth: ' + this.depth + '})" data-id="' + id + '" data-depth="' + this.depth + '" @dragenter.prevent="onDragEnter()" @dragover.prevent="onDragOver()" @drop="onDrop()" class="group relative flex items-start"><div draggable="true" @dragstart="onDragStart()" @dragend="onDragEnd()" title="drag to reorder" class="mr-1 hidden cursor-grab select-none px-0.5 pt-0.5 text-zinc-600 group-hover:flex hover:text-zinc-300">⠿</div><button @click="toggleMenu()" title="block type" class="mr-1 hidden cursor-pointer select-none px-0.5 pt-0.5 text-zinc-600 group-hover:flex hover:text-zinc-300">+</button><div contenteditable="true" spellcheck="false" data-placeholder="Type / for blocks" @focus="onFocus()" @blur="onBlur()" @input="onInput()" @keydown="onKeydown($event)" class="block-editor outline-none cursor-text py-0.5" x-data="editor({id: \'' + id + '\', type: \'text\', pageId: \'' + this.pageId + '\', depth: ' + this.depth + ', raw: \'\'})"></div><div x-show="menu" @click.away="closeSlash($root)" class="absolute left-6 top-6 z-10 mt-1 w-48 rounded-lg border border-zinc-800 bg-zinc-950 py-1 text-sm shadow-xl"><template x-for="(t, i) in blockTypesList.filter(x => x !== menuType && (!menuFilter || x.includes(menuFilter)))" :key="t"><button :class="i === menuIndex ? \'bg-zinc-800\' : \'\'" @mousedown.prevent="pickSlashType($root, t)" class="block w-full px-3 py-1 text-left text-zinc-300 hover:bg-zinc-800"><span x-text="t" class="capitalize"></span></button></template></div></div>'
		},

		placeCaret(el) {
			const range = document.createRange()
			range.selectNodeContents(el)
			range.collapse(false)
			const sel = document.getSelection()
			sel.removeAllRanges()
			sel.addRange(range)
		}
	}))

	Alpine.data('finance', (props) => ({
		id: props.id,
		pageId: props.pageId,
		pocket: props.pocket,

		init() {
			this.$el.value = this.pocket
		},

		async change() {
			this.pocket = this.$el.value
			const res = await fetch('/api/blocks/' + this.id, {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ type: 'finance', content: { finance: { pocket: this.pocket } } })
			})
			if (!res.ok) return
			const layoutEl = document.querySelector('[x-data="layout"]')
			if (layoutEl) Alpine.$data(layoutEl).loadContent(this.pageId)
		}
	}))

	Alpine.data('row', (props) => ({
		id: props.id,
		type: props.type,
		pageId: props.pageId,
		depth: props.depth,
		menu: false,
		menuFilter: '',
		menuIndex: 0,
		menuType: 'text',
		creating: false,

		toggleMenu() {
			this.menu = !this.menu
			this.menuFilter = ''
			this.menuIndex = 0
			this.menuType = this.type
		},

		onDragStart() {
			drag = { id: this.id, row: this.$el }
		},

		onDragEnd() {
			drag = null
		},

		onDragEnter() {},

		onDragOver() {},

		async onDrop() {
			if (!drag || drag.id === this.id || drag.row.parentNode !== this.$el.parentNode) return
			const rows = [...this.$el.parentNode.children].filter(el => Number(el.dataset.depth) === this.depth)
			const from = rows.indexOf(drag.row)
			const to = rows.indexOf(this.$el)
			if (from < 0 || to < 0) return

			const ids = rows.map(el => el.dataset.id)
			ids.splice(from, 1)
			ids.splice(to, 0, drag.id)

			const res = await fetch('/api/pages/' + this.pageId + '/blocks/reorder', {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(ids.map((id, i) => ({ id, order: i + 1 })))
			})
			if (!res.ok) return

			if (from < to) {
				drag.row.parentNode.insertBefore(drag.row, this.$el.nextSibling)
			} else {
				drag.row.parentNode.insertBefore(drag.row, this.$el)
			}
			drag = null
		}
	}))

	Alpine.data('pocketView', () => ({
		editing: null,

		edit(e) {
			const row = e.currentTarget.closest('tr')
			const data = JSON.parse(row.dataset.row)
			this.editing = data
			this.$refs.nameInput.value = data.name
			this.$refs.typeSelect.value = data.type
		},

		cancelEdit() {
			this.editing = null
			this.$refs.nameInput.value = ''
		},

		async add() {
			const name = this.$refs.nameInput.value.trim()
			if (!name) return
			const url = this.editing ? '/api/pockets/' + this.editing.id : '/api/pockets'
			const res = await fetch(url, {
				method: this.editing ? 'PATCH' : 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name, type: this.$refs.typeSelect.value })
			})
			if (!res.ok) return
			this.cancelEdit()
			this.reload()
		},

		async remove(id) {
			if (!confirm('Delete this pocket?')) return
			const res = await fetch('/api/pockets/' + id, { method: 'DELETE' })
			if (!res.ok) return
			this.reload()
		},

		reload() {
			const layoutEl = document.querySelector('[x-data="layout"]')
			if (layoutEl) Alpine.$data(layoutEl).loadFinance('pockets')
		}
	}))

	Alpine.data('transactionForm', () => ({
		type: 'expense',
		editing: null,

		setType(t) {
			this.type = t
		},

		typeClass(t) {
			return t === this.type
				? 'bg-zinc-700 text-white'
				: 'text-zinc-400 hover:bg-zinc-800 hover:text-white'
		},

		edit(e) {
			const row = e.currentTarget.closest('tr')
			const data = JSON.parse(row.dataset.row)
			this.editing = data
			this.setType(data.type)
			this.$refs.titleInput.value = data.title
			this.$refs.amountInput.value = data.amount
			this.$refs.dateInput.value = data.date
			if (data.from) this.$refs.fromSelect.value = data.from
			if (data.to) this.$refs.toSelect.value = data.to
		},

		cancelEdit() {
			this.editing = null
			this.$refs.titleInput.value = ''
			this.$refs.amountInput.value = ''
		},

		refresh() {
			const params = new URLSearchParams()
			const pocket = this.$refs.filterPocket.value
			const from = this.$refs.filterFrom.value
			const to = this.$refs.filterTo.value
			if (pocket) params.set('pocket', pocket)
			if (from) params.set('from', from)
			if (to) params.set('to', to)
			const qs = params.toString()
			const layoutEl = document.querySelector('[x-data="layout"]')
			if (layoutEl) Alpine.$data(layoutEl).loadFinance('transactions' + (qs ? '?' + qs : ''))
		},

		filter() {
			this.refresh()
		},

		clearFilter() {
			this.$refs.filterPocket.value = ''
			this.$refs.filterFrom.value = ''
			this.$refs.filterTo.value = ''
			this.filter()
		},

		async create() {
			const title = this.$refs.titleInput.value.trim()
			const amount = Number(this.$refs.amountInput.value)
			if (!title || !amount || amount <= 0) return
			const date = this.$refs.dateInput.value || new Date().toISOString().slice(0, 10)
			const body = {
				title,
				type: this.type,
				amount,
				date
			}
			if (this.type !== 'income') body.from_pocket_id = this.$refs.fromSelect.value || (this.editing ? this.editing.from : null) || null
			if (this.type !== 'expense') body.to_pocket_id = this.$refs.toSelect.value || (this.editing ? this.editing.to : null) || null
			const url = this.editing ? '/api/transactions/' + this.editing.id : '/api/transactions'
			const res = await fetch(url, {
				method: this.editing ? 'PATCH' : 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(body)
			})
			if (!res.ok) return
			this.reload()
		},

		async remove(id) {
			if (!confirm('Delete this transaction?')) return
			const res = await fetch('/api/transactions/' + id, { method: 'DELETE' })
			if (!res.ok) return
			this.reload()
		},

		reload() {
			const layoutEl = document.querySelector('[x-data="layout"]')
			if (layoutEl) Alpine.$data(layoutEl).loadFinance('transactions')
		}
	}))

	Alpine.data('trashView', () => ({
		async restorePocket(id) {
			const res = await fetch('/api/pockets/' + id + '/restore', { method: 'POST' })
			if (!res.ok) return
			this.reload()
		},

		async restoreTransaction(id) {
			const res = await fetch('/api/transactions/' + id + '/restore', { method: 'POST' })
			if (!res.ok) return
			this.reload()
		},

		reload() {
			const layoutEl = document.querySelector('[x-data="layout"]')
			if (layoutEl) Alpine.$data(layoutEl).loadFinance('trash')
		}
	}))

	Alpine.data('transactionTags', (props) => ({
		id: props.id,
		adding: false,

		toggleAdd() {
			this.adding = !this.adding
			if (this.adding) {
				this.$nextTick(() => this.$refs.tagInput.focus())
			}
		},

		async add() {
			const name = this.$refs.tagInput.value.trim()
			if (!name) return
			const res = await fetch('/api/tags', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name, color: '#3b82f6' })
			})
			if (!res.ok) return
			const tag = await res.json()
			const attach = await fetch('/api/tags/' + tag.id + '/attach', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ target_id: this.id, target_type: 'transaction' })
			})
			if (!attach.ok) return
			this.reload()
		},

		async detach(tagId) {
			const res = await fetch('/api/tags/' + tagId + '/detach', {
				method: 'DELETE',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ target_id: this.id, target_type: 'transaction' })
			})
			if (!res.ok) return
			this.reload()
		},

		reload() {
			const formEl = this.$el.closest('[x-data="transactionForm()"]')
			if (!formEl) return
			Alpine.$data(formEl).refresh()
		}
	}))

	Alpine.data('pageTags', (props) => ({
		pageId: props.pageId,
		adding: false,

		toggleAdd() {
			this.adding = !this.adding
			if (this.adding) {
				this.$nextTick(() => this.$refs.tagInput.focus())
			}
		},

		async add() {
			const name = this.$refs.tagInput.value.trim()
			if (!name) return
			const res = await fetch('/api/tags', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name, color: '#3b82f6' })
			})
			if (!res.ok) return
			const tag = await res.json()
			const attach = await fetch('/api/tags/' + tag.id + '/attach', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ target_id: this.pageId, target_type: 'page' })
			})
			if (!attach.ok) return
			this.reload()
		},

		async detach(tagId) {
			const res = await fetch('/api/tags/' + tagId + '/detach', {
				method: 'DELETE',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ target_id: this.pageId, target_type: 'page' })
			})
			if (!res.ok) return
			this.reload()
		},

		reload() {
			const layoutEl = document.querySelector('[x-data="layout"]')
			if (layoutEl) Alpine.$data(layoutEl).loadContent(this.pageId)
		}
	}))

	Alpine.data('pageTitle', (props) => ({
		pageId: props.pageId,
		title: props.title,
		saveTimer: null,

		onInput() {
			clearTimeout(this.saveTimer)
			this.saveTimer = setTimeout(() => this.save(), 300)
		},

		onKeydown(e) {
			if (e.key === 'Enter') {
				e.preventDefault()
				clearTimeout(this.saveTimer)
				this.save()
				this.$el.blur()
			}
		},

		onBlur() {
			if (this.$el.innerText.trim() === this.title) return
			clearTimeout(this.saveTimer)
			this.save()
		},

		async save() {
			const title = this.$el.innerText.trim()
			const res = await fetch('/api/pages/' + this.pageId, {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ title })
			})
			if (!res.ok) return
			this.title = title
			const layoutEl = document.querySelector('[x-data="layout"]')
			if (layoutEl) Alpine.$data(layoutEl).fetchPages(1)
		}
	}))
})
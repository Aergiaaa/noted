document.addEventListener('alpine:init', () => {
	let drag = null

	Alpine.data('layout', () => ({
		sidebarOpen: false,
		navbarVisible: true,
		pages: [],
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
		},

		async fetchPages(page) {
			const res = await fetch('/api/pages?page=' + page + '&limit=10')
			if (!res.ok) return
			const data = await res.json()
			this.pages = data.pages
			this.totalPages = data.total_pages
			this.currentPaginationPage = page
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
			this.$el.innerHTML = this.escaped(this.raw).replace(/\n/g, '<br>')
			this.placeCaret(this.$el)
		},

		async onBlur() {
			const text = this.$el.innerText.replace(/\n$/, '')
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

		onInput() {
			clearTimeout(this.saveTimer)
			this.saveTimer = setTimeout(() => this.save(), 300)
		},

		async save() {
			const text = this.$el.innerText.replace(/\n$/, '')
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

		async onKeydown(e) {
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
			return '<div x-data="row({id: \'' + id + '\', pageId: \'' + this.pageId + '\', depth: ' + this.depth + '})" data-id="' + id + '" data-depth="' + this.depth + '" @dragenter.prevent="onDragEnter()" @dragover.prevent="onDragOver()" @drop="onDrop()" class="group flex items-start"><div draggable="true" @dragstart="onDragStart()" @dragend="onDragEnd()" title="drag to reorder" class="mr-1 hidden cursor-grab select-none px-0.5 pt-0.5 text-zinc-600 group-hover:flex hover:text-zinc-300">⠿</div><div contenteditable="true" spellcheck="false" @focus="onFocus()" @blur="onBlur()" class="outline-none cursor-text py-0.5" x-data="editor({id: \'' + id + '\', type: \'text\', pageId: \'' + this.pageId + '\', depth: ' + this.depth + ', raw: \'\'})"></div></div>'
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

	Alpine.data('row', (props) => ({
		id: props.id,
		pageId: props.pageId,
		depth: props.depth,

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
		async add() {
			const name = this.$refs.nameInput.value.trim()
			if (!name) return
			const res = await fetch('/api/pockets', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name, type: this.$refs.typeSelect.value })
			})
			if (!res.ok) return
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

		setType(t) {
			this.type = t
		},

		typeClass(t) {
			return t === this.type
				? 'bg-zinc-700 text-white'
				: 'text-zinc-400 hover:bg-zinc-800 hover:text-white'
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
			if (this.type !== 'income') body.from_pocket_id = this.$refs.fromSelect.value || null
			if (this.type !== 'expense') body.to_pocket_id = this.$refs.toSelect.value || null
			const res = await fetch('/api/transactions', {
				method: 'POST',
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
		saveTimer: null,

		onInput() {
			clearTimeout(this.saveTimer)
			this.saveTimer = setTimeout(() => this.save(), 300)
		},

		onKeydown(e) {
			if (e.key === 'Enter') {
				e.preventDefault()
				this.save()
				this.$el.blur()
			}
		},

		async save() {
			const title = this.$el.innerText.trim()
			const res = await fetch('/api/pages/' + this.pageId, {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ title })
			})
			if (!res.ok) return
			const layoutEl = document.querySelector('[x-data="layout"]')
			if (layoutEl) Alpine.$data(layoutEl).fetchPages(1)
		}
	}))
})
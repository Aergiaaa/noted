document.addEventListener('alpine:init', () => {
	let drag = null

	Alpine.data('layout', () => ({
		sidebarOpen: false,
		navbarVisible: true,
		pages: [],
		totalPages: 1,
		currentPaginationPage: 1,
		activePageId: null,

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

		async createPage() {
			const res = await fetch('/api/pages', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ title: 'Untitled' })
			})
			if (!res.ok) return
			const data = await res.json()
			await this.fetchPages(1)
			this.loadContent(data.id)
		},

		async loadContent(pageId) {
			this.activePageId = pageId
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
		saveTimer: null,

		onInput() {
			clearTimeout(this.saveTimer)
			this.saveTimer = setTimeout(() => this.save(), 300)
		},

		async save() {
			const text = this.$el.innerText.replace(/\n$/, '')
			const res = await fetch('/api/blocks/' + this.id, {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ type: this.type, content: { text } })
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
			return '<div x-data="row({id: \'' + id + '\', pageId: \'' + this.pageId + '\', depth: ' + this.depth + '})" data-id="' + id + '" data-depth="' + this.depth + '" @dragenter.prevent="onDragEnter()" @dragover.prevent="onDragOver()" @drop="onDrop()" class="group flex items-start"><div draggable="true" @dragstart="onDragStart()" @dragend="onDragEnd()" title="drag to reorder" class="mr-1 hidden cursor-grab select-none px-0.5 pt-0.5 text-zinc-600 group-hover:flex hover:text-zinc-300">⠿</div><div contenteditable="true" spellcheck="false" class="outline-none cursor-text py-0.5" x-data="editor({id: \'' + id + '\', type: \'text\', pageId: \'' + this.pageId + '\', depth: ' + this.depth + '})"></div></div>'
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
})
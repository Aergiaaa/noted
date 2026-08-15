document.addEventListener('alpine:init', () => {
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
			wrapper.innerHTML = '<div contenteditable="true" spellcheck="false" class="outline-none cursor-text py-0.5" x-data="editor({id: \'' + data.id + '\', type: \'text\', pageId: \'' + this.pageId + '\'})"></div>'
			const node = wrapper.firstElementChild
			this.$el.parentNode.insertBefore(node, this.$el.nextSibling)
			Alpine.initTree(node)
			node.focus()
			this.placeCaret(node)
		},

		async deleteSelf() {
			const prev = this.$el.previousElementSibling
			const res = await fetch('/api/blocks/' + this.id, {
				method: 'DELETE'
			})
			if (!res.ok) return
			this.$el.remove()
			if (prev) {
				prev.focus()
				this.placeCaret(prev)
			}
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
})
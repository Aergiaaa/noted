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

		async loadContent(pageId) {
			this.activePageId = pageId
			this.sidebarOpen = false
			const res = await fetch('/pages/' + pageId + '/fragment')
			const html = await res.text()
			const target = document.getElementById('main-content')
			target.innerHTML = html
			Alpine.initTree(target)
		}
	}))
})

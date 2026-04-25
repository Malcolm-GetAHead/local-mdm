/* Local MDM Dashboard — consolidated JS (event delegation for HTMX content swaps) */
(function(){
    // ── Dark mode (runs once on load) ──
    if(localStorage.getItem('darkMode')==='true') document.documentElement.classList.add('dark');

    // ── Dark mode toggle ──
    document.addEventListener('click', function(e) {
        if (e.target.closest('#dark-toggle')) {
            document.documentElement.classList.toggle('dark');
            localStorage.setItem('darkMode', document.documentElement.classList.contains('dark'));
        }
    });

    // ── Mobile hamburger menu ──
    document.addEventListener('click', function(e) {
        var sidebar = document.getElementById('sidebar');
        var backdrop = document.getElementById('sidebar-backdrop');
        if (!sidebar || !backdrop) return;
        if (e.target.closest('#menu-toggle')) {
            sidebar.classList.toggle('-translate-x-full');
            backdrop.classList.toggle('hidden');
        }
        if (e.target === backdrop) {
            sidebar.classList.add('-translate-x-full');
            backdrop.classList.add('hidden');
        }
    });

    // ── Sidebar active highlight on HTMX navigation ──
    document.body.addEventListener('htmx:pushedIntoHistory', function() {
        var path = window.location.pathname.replace(/\/$/, '');
        document.querySelectorAll('#sidebar a').forEach(function(a) {
            var href = a.getAttribute('href').replace(/\/$/, '');
            var isActive = (path === href) || (href === '/dashboard' && path === '/dashboard');
            a.classList.toggle('bg-gray-100', isActive);
            a.classList.toggle('dark:bg-gray-700', isActive);
            a.classList.toggle('font-medium', isActive);
        });
    });

    // ── Audit log expand ──
    document.addEventListener('click', function(e) {
        var row = e.target.closest('[data-expand="audit"]');
        if (!row) return;
        var detail = row.nextElementSibling;
        if (detail && detail.classList.contains('detail-row')) {
            detail.classList.toggle('hidden');
            var arrow = row.querySelector('.expand-arrow');
            if (arrow) arrow.textContent = detail.classList.contains('hidden') ? '▶' : '▼';
        }
    });

    // ── Device detail tabs ──
    document.addEventListener('click', function(e) {
        var btn = e.target.closest('.tab-btn');
        if (!btn) return;
        var container = btn.closest('.card');
        if (!container) return;
        var tab = btn.getAttribute('data-tab');
        container.querySelectorAll('.tab-btn').forEach(function(b) {
            b.classList.remove('border-blue-600','text-blue-600','dark:text-blue-400');
            b.classList.add('border-transparent','text-gray-500');
        });
        btn.classList.add('border-blue-600','text-blue-600','dark:text-blue-400');
        btn.classList.remove('border-transparent','text-gray-500');
        container.querySelectorAll('.tab-panel').forEach(function(p) { p.classList.add('hidden'); });
        var target = document.getElementById('tab-' + tab);
        if (target) target.classList.remove('hidden');
    });

    // ── Group create form toggle ──
    document.addEventListener('click', function(e) {
        if (e.target.id === 'toggle-create-form' || e.target.closest('#toggle-create-form')) {
            var form = document.getElementById('create-group-form');
            if (form) form.classList.toggle('hidden');
        }
    });

    // ── Group detail edit/cancel ──
    document.addEventListener('click', function(e) {
        var display = document.getElementById('group-display');
        var form = document.getElementById('group-edit-form');
        if (!display || !form) return;
        if (e.target.id === 'edit-group-btn' || e.target.closest('#edit-group-btn')) {
            display.classList.add('hidden'); form.classList.remove('hidden');
        }
        if (e.target.id === 'cancel-edit-btn' || e.target.closest('#cancel-edit-btn')) {
            form.classList.add('hidden'); display.classList.remove('hidden');
        }
    });

    // ── Chart hover highlight ──
    document.addEventListener('mouseenter', function(e) {
        if (!e.target || !e.target.closest) return;
        var el = e.target.closest('[data-slice]');
        if (!el) return;
        var chart = el.closest('[data-chart]');
        if (!chart) return;
        var idx = el.getAttribute('data-slice');
        chart.querySelectorAll('[data-slice]').forEach(function(s) {
            var match = s.getAttribute('data-slice') === idx;
            if (s.tagName === 'path') { s.style.opacity = match ? '0.6' : '1'; }
            else { s.style.opacity = match ? '1' : '0.5'; var sp = s.querySelector('span'); if (sp) sp.style.fontWeight = match ? 'bold' : ''; }
        });
    }, true);
    document.addEventListener('mouseleave', function(e) {
        if (!e.target || !e.target.closest) return;
        var el = e.target.closest('[data-slice]');
        if (!el) return;
        var chart = el.closest('[data-chart]');
        if (!chart) return;
        chart.querySelectorAll('[data-slice]').forEach(function(s) {
            s.style.opacity = ''; var sp = s.querySelector('span'); if (sp) sp.style.fontWeight = '';
        });
    }, true);

    // ── Debounced filter inputs ──
    // Handles: member-filter, settings-filter, group-search/group-select, device-search/device-select
    var filterTimers = {};

    function applyMemberFilter() {
        var el = document.getElementById('member-filter');
        if (!el || !el.value) return;
        var q = el.value.toLowerCase();
        document.querySelectorAll('[data-device-name]').forEach(function(row) {
            row.classList.toggle('hidden', !row.getAttribute('data-device-name').toLowerCase().includes(q));
        });
    }

    // Re-apply member filter after HTMX swaps the tbody
    document.body.addEventListener('htmx:afterSwap', function(e) {
        if (e.detail.target && e.detail.target.id === 'member-tbody') {
            applyMemberFilter();
        }
    });

    document.addEventListener('input', function(e) {
        var el = e.target;
        // Member filter (group detail)
        if (el.id === 'member-filter') {
            clearTimeout(filterTimers.member);
            filterTimers.member = setTimeout(applyMemberFilter, 200);
        }
        // Settings filter (policy form)
        if (el.id === 'settings-filter') {
            clearTimeout(filterTimers.settings);
            filterTimers.settings = setTimeout(function() {
                var q = el.value.toLowerCase();
                document.querySelectorAll('[data-setting]').forEach(function(s) {
                    s.classList.toggle('hidden', !s.getAttribute('data-setting').toLowerCase().includes(q));
                });
                document.querySelectorAll('[data-category]').forEach(function(cat) {
                    cat.classList.toggle('hidden', cat.querySelectorAll('[data-setting]:not(.hidden)').length === 0);
                });
            }, 200);
        }
        // Policy assign search (group-search, device-search)
        if (el.id === 'group-search' || el.id === 'device-search') {
            var selectId = el.id === 'group-search' ? 'group-select' : 'device-select';
            clearTimeout(filterTimers[el.id]);
            filterTimers[el.id] = setTimeout(function() {
                var q = el.value.toLowerCase();
                var select = document.getElementById(selectId);
                if (!select) return;
                Array.from(select.options).forEach(function(opt) {
                    opt.hidden = q.length > 0 && !opt.getAttribute('data-search').toLowerCase().includes(q);
                });
            }, 300);
        }
    });

    // Prevent Enter from submitting filter inputs
    document.addEventListener('keydown', function(e) {
        if (e.key === 'Enter' && (e.target.id === 'member-filter' || e.target.id === 'settings-filter' ||
            e.target.id === 'group-search' || e.target.id === 'device-search')) {
            e.preventDefault();
        }
    });

    // ── Toast notifications ──
    function showToast(message, type) {
        var container = document.getElementById('toast-container');
        if (!container) return;
        var colors = type === 'error' ? 'bg-red-600' : 'bg-green-600';
        var toast = document.createElement('div');
        toast.className = colors + ' text-white px-4 py-2 rounded-lg shadow-lg text-sm transition-opacity duration-300';
        toast.textContent = message;
        container.appendChild(toast);
        setTimeout(function() { toast.style.opacity = '0'; }, 2500);
        setTimeout(function() { toast.remove(); }, 3000);
    }
    // Listen for HX-Trigger events from server
    document.body.addEventListener('showToast', function(e) {
        showToast(e.detail.message || 'Done', e.detail.type || 'success');
    });
    // Auto-toast on successful HTMX actions (non-navigation)
    document.body.addEventListener('htmx:afterRequest', function(e) {
        var target = e.detail.target;
        if (!target) return;
        var id = target.id || '';
        // Toast for device actions
        if (id === 'device-actions' && e.detail.successful) {
            showToast('Action sent successfully', 'success');
        }
    });
})();

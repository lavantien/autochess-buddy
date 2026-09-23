// the one sanctioned wiring beyond declarative attributes: focus lands on the
// autofocused field after a swap, else on the neighbor a delete captured.
// captures hold element ids, never nodes: the swap detaches the captured
// node, so the neighbor re-resolves inside the fresh dom. the autofocus
// attribute is consumed on focus so a stale one can never win a later swap.
function acFocusable(el) {
	if (!el) { return null; }
	if (el.matches('button, [href], input, select, textarea, summary')) { return el; }
	return el.querySelector('button, [href], input, select, textarea, summary');
}

// delete slot: focus the previous slot's cell, else the next, else the hero form.
function acCaptureSlot(ctl) {
	var cell = ctl.closest('[id^="slot-"]');
	var n = cell.previousElementSibling;
	while (n && !n.id) { n = n.previousElementSibling; }
	if (!n) {
		n = cell.nextElementSibling;
		while (n && !n.id) { n = n.nextElementSibling; }
	}
	window.__acNext = n ? n.id : ctl.closest('[id^="grid-"]').id.replace('grid-', 'heroform-');
}

// delete lineup: focus the next card, else the add form.
function acCaptureCard(ctl) {
	var card = ctl.closest('[id^="card-"]');
	var n = card.nextElementSibling;
	while (n && n.id.indexOf('card-') !== 0) { n = n.nextElementSibling; }
	window.__acNext = n ? n.id : ctl.closest('[id^="cards-"]').id.replace('cards-', 'addform-');
}

document.addEventListener('htmx:after:swap', function () {
	if (window.__acNext) {
		var id = window.__acNext;
		window.__acNext = null;
		var t = acFocusable(document.getElementById(id));
		if (t) { t.focus(); return; }
	}
	var a = document.querySelector('[data-autofocus]');
	if (a) { a.focus(); a.removeAttribute('data-autofocus'); }
});

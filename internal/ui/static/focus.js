// the one sanctioned wiring beyond declarative attributes: focus lands on the
// autofocused field after a swap, else on the neighbor a delete captured.
document.addEventListener('htmx:after:swap', function () {
	var a = document.querySelector('[data-autofocus]');
	if (a) { a.focus(); return; }
	if (window.__acNext) { window.__acNext.focus(); window.__acNext = null; }
});

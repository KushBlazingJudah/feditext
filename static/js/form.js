(function(){
function clamp(min, x, max) {
	return Math.max(min, Math.min(x, max))
}

document.addEventListener("DOMContentLoaded", ()=>{
	let header = document.getElementById("pfheader");
	let close = document.getElementById("pfclose");
	let open = document.getElementById("pfopen");
	let form = document.getElementById("postForm");

	open.onclick = (e) => {
		e = e || window.event;
		e.preventDefault();

		form.classList.add("floating");
	};

	window.onresize = (e) => {
		form.style.top = `${clamp(0, form.offsetTop, window.screen.height - form.clientHeight)}px`;
		form.style.left = `${clamp(0, form.offsetLeft, window.screen.width - form.clientWidth)}px`;
	};

	header.onmousedown = (e) => {
		let offX = e.clientX - form.offsetLeft;
		let offY = e.clientY - form.offsetTop;

		e = e || window.event;
		e.preventDefault();

		if (e.target == close) {
			form.classList.remove("floating");
			return;
		}

		document.onmousemove = (e) => {
			e = e || window.event;
			e.preventDefault();

			x = offX - e.clientX;
			y = offY - e.clientY;

			form.style.top = `${clamp(0, e.clientY - offY, window.screen.height - form.clientHeight)}px`;
			form.style.left = `${clamp(0, e.clientX - offX, window.screen.width - form.clientWidth)}px`;
		};

		document.onmouseup = (e) => {
			e = e || window.event;
			e.preventDefault();

			document.onmousemove = null;
			document.onmouseup = null;
		};
	};
}, false);
})();

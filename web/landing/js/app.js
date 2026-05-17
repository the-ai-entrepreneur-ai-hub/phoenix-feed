const observer = new IntersectionObserver((entries) => {
  entries.forEach((entry) => {
    if (entry.isIntersecting) {
      entry.target.classList.add('visible');
    }
  });
}, { threshold: 0.15, rootMargin: '0px 0px -50px 0px' });

document.querySelectorAll('.reveal').forEach((el) => {
  observer.observe(el);
});

setTimeout(() => {
  document.querySelectorAll('section:first-of-type .reveal').forEach((el) => {
    el.classList.add('visible');
  });
}, 100);

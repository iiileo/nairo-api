(function () {
  if (window.lucide && typeof window.lucide.createIcons === 'function') {
    window.lucide.createIcons();
  }

  var topnav = document.getElementById('topnav');
  function onScroll() {
    if (!topnav) return;
    topnav.classList.toggle('is-scrolled', window.scrollY > 24);
  }
  onScroll();
  window.addEventListener('scroll', onScroll, { passive: true });

  if ('IntersectionObserver' in window) {
    var io = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry) {
          if (entry.isIntersecting) {
            entry.target.classList.add('in');
            io.unobserve(entry.target);
          }
        });
      },
      { rootMargin: '0px 0px -8% 0px', threshold: 0.12 },
    );
    document.querySelectorAll('.is-reveal').forEach(function (el) {
      io.observe(el);
    });
  } else {
    document.querySelectorAll('.is-reveal').forEach(function (el) {
      el.classList.add('in');
    });
  }

  var form = document.getElementById('demo-form');
  var statusEl = document.getElementById('demo-status');
  var amountEl = document.getElementById('amount');
  var channelEl = document.getElementById('channel');
  var ackEl = document.getElementById('ack');
  var submitEl = document.getElementById('submit');
  var errEl = document.getElementById('demo-err');
  if (!form || !statusEl || !amountEl || !channelEl || !ackEl || !submitEl || !errEl) return;

  function syncSubmit() {
    submitEl.disabled = !ackEl.checked;
  }
  ackEl.addEventListener('change', syncSubmit);

  document.querySelectorAll('.presets button').forEach(function (btn) {
    btn.addEventListener('click', function () {
      document.querySelectorAll('.presets button').forEach(function (b) {
        b.classList.remove('active');
      });
      btn.classList.add('active');
      amountEl.value = (Number(btn.getAttribute('data-cents')) / 100).toFixed(2);
    });
  });

  document.querySelectorAll('.channel-pick').forEach(function (btn) {
    btn.addEventListener('click', function () {
      document.querySelectorAll('.channel-pick').forEach(function (b) {
        b.classList.remove('active');
        b.setAttribute('aria-pressed', 'false');
      });
      btn.classList.add('active');
      btn.setAttribute('aria-pressed', 'true');
      channelEl.value = btn.getAttribute('data-channel') || 'wechat';
    });
  });

  fetch('/api/v1/public/demo-pay/status')
    .then(function (r) {
      return r.json();
    })
    .then(function (d) {
      if (!d.configured) {
        statusEl.textContent = '平台尚未配置收款通道，试付暂不可用。';
        return;
      }
      statusEl.textContent = d.warning || '通道已就绪，可发起试付。';
      form.hidden = false;
      if (window.lucide) window.lucide.createIcons();
    })
    .catch(function () {
      statusEl.textContent = '无法检查试付状态，请稍后刷新。';
    });

  form.addEventListener('submit', function (e) {
    e.preventDefault();
    errEl.hidden = true;
    if (!ackEl.checked) return;
    var yuan = Number(amountEl.value);
    if (!isFinite(yuan) || yuan <= 0) {
      errEl.textContent = '请输入有效金额';
      errEl.hidden = false;
      return;
    }
    var cents = Math.round(yuan * 100);
    submitEl.disabled = true;
    submitEl.textContent = '创建中…';
    fetch('/api/v1/public/demo-pay', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ amount_cents: cents, channel: channelEl.value }),
    })
      .then(function (r) {
        return r.json().then(function (d) {
          return { ok: r.ok, d: d };
        });
      })
      .then(function (x) {
        if (!x.ok) {
          errEl.textContent = x.d.message || x.d.error || '创建失败';
          errEl.hidden = false;
          submitEl.disabled = !ackEl.checked;
          submitEl.innerHTML = '发起试付 <i data-lucide="arrow-up-right" class="icon"></i>';
          if (window.lucide) window.lucide.createIcons();
          return;
        }
        location.href = x.d.pay_page || '/demo/' + x.d.order_id;
      })
      .catch(function () {
        errEl.textContent = '网络错误';
        errEl.hidden = false;
        submitEl.disabled = !ackEl.checked;
        submitEl.innerHTML = '发起试付 <i data-lucide="arrow-up-right" class="icon"></i>';
        if (window.lucide) window.lucide.createIcons();
      });
  });
})();

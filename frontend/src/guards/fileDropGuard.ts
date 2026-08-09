/**
 * 全局文件拖放防线
 *
 * WebView2 把「拖进窗口的文件」当导航请求处理，且这套行为在 wails build
 * 的发行版同样生效——只是没有右键菜单，不容易被发现。实测症状：
 *
 *   - 拖任意文件到界面上非导入区的地方，会闪一个 Windows 透明弹窗
 *   - 拖文本类文件会另开 Edge 原生的文本查看器窗口
 *   - 该查看器里某些按钮会让 WebView2 崩溃，宿主进程一并倒掉
 *
 * 即用户拖错位置就能弄崩盒子，而这条路径绕过了本工具的全部校验——
 * `DropZone` 只认压缩包，可它管不到自己以外的区域。整个界面除了那一块
 * 之外，处处都是没设防的入口。
 *
 * 故防线要建在 document 上：默认全部拒绝，只给 DropZone 开一扇门。
 * 这个方向与「白名单优于黑名单」一致——列举「哪里能拖」是有限的，
 * 列举「哪些文件类型危险」则永远列不完。
 *
 * NOTE: 不影响 DropZone。它的 handler 在冒泡途中先跑，等事件到达
 * document 时文件早已交给业务逻辑，此处再 preventDefault 只是重复
 * 关掉浏览器默认动作，两者不冲突。
 *
 * NOTE: 也不影响 `<input type="file">` 的点击选择路径——那条路走的是
 * 系统文件对话框，与拖放事件无关。
 *
 * ── 链接拖拽防线 ──
 *
 * WebView2 对 `<a href>` 元素长按后拖拽会触发原生的 link drag，弹出灰色
 * 半透明 URL 预览浮层（第一行为链接文字，第二行为完整 href）。发行版同样
 * 复现。RouterLink 渲染成真实 `<a>` 元素，侧栏条目与顶栏页签均受影响。
 *
 * `draggable="false"` 对带 href 的 `<a>` 无效——浏览器规范里链接元素
 * 的可拖拽性由 href 决定，属性不能覆盖。唯一有效手段是在 dragstart
 * 捕获阶段调用 preventDefault。
 *
 * 故在此追加 dragstart 捕获监听，一次拦截所有元素，不逐一处理各路由组件。
 */

/**
 * 装上防线。在 app.mount() 之前调用一次即可，无需卸载。
 *
 * 必须两个事件都挡：
 *   - 只挡 drop 不挡 dragover，WebView 在 dragover 阶段就已接管，
 *     drop 根本不会派发到页面上
 *   - 只挡 dragover 不挡 drop，则文件仍会被当导航打开
 *
 * dragstart 同样走捕获阶段，理由同上：安全兜底不该依赖别处的实现细节，
 * 即便将来某处写了 stopPropagation，防线依然先于它执行。
 */
export function installFileDropGuard() {
  window.addEventListener('dragover', preventUnlessAllowed, { capture: true })
  window.addEventListener('drop', preventUnlessAllowed, { capture: true })
  // 拦截链接元素的拖拽起始，防止 WebView2 弹出原生 URL 预览浮层。
  // 不过滤 target 类型——整个界面里任何元素被拖拽都没有意义，
  // 唯一合法的拖放入口是 DropZone（走文件拖入路径，不是 dragstart）。
  window.addEventListener('dragstart', preventUnlessAllowed, { capture: true })
}

/**
 * 拒绝浏览器对拖放的默认处理。
 *
 * 不区分目标元素：允许拖入的区域自己会 preventDefault 并读取
 * `dataTransfer`，本函数只负责关掉浏览器那套「打开文件」的默认动作。
 * 换言之这里挡的不是「事件传递」，而是「浏览器接管」——业务逻辑照旧收到
 * 事件，DropZone 因此不受影响。
 */
function preventUnlessAllowed(event: DragEvent) {
  event.preventDefault()
}

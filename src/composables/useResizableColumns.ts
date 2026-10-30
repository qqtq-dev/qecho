import { ref } from 'vue'

export interface ResizableColumn {
  title: string
  width?: number
  resizable?: boolean
  minWidth?: number
  [key: string]: unknown
}

const CELL_PADDING = 34
// 吸收列（末尾无宽列）多为图标按钮：浏览器按其内容决定的最小宽大于探针值，预算里留出余量
const ABSORBER_GUARD = 48

/**
 * 可拖拽列宽表格（分隔线语义，单元格恒单行）：
 * - 把手拖动 = 移动该列与下一列的分隔线：拖窄时本列缩、空间归下一列；
 *   拖宽时先压缩右侧各列，右侧到底再从左侧各列匀出——永远可以继续拖
 * - 所有列最小宽一律按内容单行实测（配合单元格 nowrap）：文字永不换行、永不截断；
 *   各列内容总宽超过容器时，表格内出现横向滚动条
 * - 吸收列（最后一列，无宽度）吸收舍入误差与窗口变化
 * - fitColumns() 供数据变化后调用：把窄于内容宽度的列补齐到单行宽度
 * - 容器：table-layout 固定布局下使用
 */
export function useResizableColumns(initial: ResizableColumn[]) {
  const lastIdx = initial.length - 1
  const columns = ref<ResizableColumn[]>(
    initial.map((c, i) => ({ ...c, resizable: i !== lastIdx })),
  )

  // 测量某一列的内容自然宽度（把单元格内容以 nowrap 方式克隆后量宽）
  function measureColumnMin(table: HTMLElement, idx: number): number {
    let max = 0
    const probeIn = (host: HTMLElement): number => {
      const probe = document.createElement('span')
      probe.style.cssText =
        'position:absolute;visibility:hidden;white-space:nowrap;width:max-content;left:-9999px;top:0;'
      probe.innerHTML = host.innerHTML
      host.appendChild(probe)
      const w = probe.getBoundingClientRect().width
      probe.remove()
      return w
    }
    table.querySelectorAll('tbody tr').forEach((row) => {
      const cell = row.children[idx] as HTMLElement | undefined
      if (cell) max = Math.max(max, probeIn(cell))
    })
    const th = table.querySelectorAll('thead tr th')[idx] as HTMLElement | undefined
    if (th) {
      const titleEl = (th.querySelector('.ant-table-column-title') || th) as HTMLElement
      max = Math.max(max, probeIn(titleEl))
    }
    return Math.ceil(max) + CELL_PADDING
  }

  // 吸收列收口：容器扣除其余列后装不下吸收列最小宽时，把它显式钉在最小宽上
  // （表格总宽超出容器，由 .ant-table-content 的横向滚动条兜底）；否则保持无宽度弹性吸收
  function settleAbsorber(table: HTMLElement, containerW: number) {
    const fixedSum = columns.value.slice(0, lastIdx).reduce((s, c) => s + (Number(c.width) || 0), 0)
    const absorberMin = measureColumnMin(table, lastIdx)
    columns.value[lastIdx].width =
      containerW - fixedSum < absorberMin ? absorberMin : undefined
  }

  function onResizeColumn(width: number, column: ResizableColumn) {
    const table = document.querySelector<HTMLElement>('.ant-table table')
    if (!table) return
    const idx = columns.value.findIndex((c) => c.title === column.title)
    if (idx < 0 || idx === lastIdx) return
    const wrapper = table.closest('.ant-table-wrapper')
    const containerW = (wrapper ?? table).clientWidth
    if (containerW <= 0) return

    // 所有列最小宽一律按内容单行实测：文字永不换行、永不截断。
    // 吸收列是图标按钮组，浏览器实测最小宽比探针值大，按余量预留：
    // 否则一路拖宽时尾列被压穿，表格宽出容器又冒出横向滚动条
    const mins = columns.value.map((_, i) => measureColumnMin(table, i))
    mins[lastIdx] += ABSORBER_GUARD

    // 当前有效宽度：吸收列（最后一列，无显式宽度）= 容器 - 其余列之和
    const eff = columns.value.map((c) => Number(c.width) || 0)
    const fixedSum = eff.reduce((s, v, i) => (i === lastIdx ? s : s + v), 0)
    eff[lastIdx] = Math.max(containerW - fixedSum, mins[lastIdx])

    // 拖拽列目标宽（不低于自身最小宽）
    const requested = Math.max(width, mins[idx])
    let delta = requested - eff[idx]
    eff[idx] = requested

    if (delta > 0) {
      // 拖宽：先压缩右侧各列（到各自最小宽为止）
      for (let i = idx + 1; i <= lastIdx && delta > 0; i++) {
        const avail = eff[i] - mins[i]
        const take = Math.min(delta, avail)
        eff[i] -= take
        delta -= take
      }
      // 右侧到底仍不够：从左侧各列匀出
      for (let i = 0; i < idx && delta > 0; i++) {
        const avail = eff[i] - mins[i]
        const take = Math.min(delta, avail)
        eff[i] -= take
        delta -= take
      }
      // 全部到最小宽仍不够：拖拽列按缺口回弹
      eff[idx] -= delta
    } else if (delta < 0) {
      // 拖窄：腾出的空间全部归入紧邻的下一列
      eff[idx + 1] -= delta
    }

    // 写回；吸收列由 settleAbsorber 按需钉住/恢复弹性
    columns.value.forEach((c, i) => {
      if (i === lastIdx) return
      c.width = Math.round(eff[i])
    })
    settleAbsorber(table, containerW)
  }

  /**
   * 数据变化后调用：把窄于内容单行宽度的列补齐到内容宽度，
   * 保证文字实时完整单行显示（只放宽不收窄，不影响用户此前的拖拽结果）。
   * 补宽以容器宽度为预算上限：不得把表格补到宽出容器，否则会重新出现横向滚动条。
   * 需在 DOM 渲染完成后调用（如 await nextTick() 之后）。
   */
  function fitColumns() {
    const table = document.querySelector<HTMLElement>('.ant-table table')
    if (!table) return
    const wrapper = table.closest('.ant-table-wrapper')
    const containerW = (wrapper ?? table).clientWidth
    if (containerW <= 0) return
    // 补宽预算：容器宽减去吸收列最小宽。吸收列是图标按钮组，浏览器实测最小宽
    // 比探针测量值略大，留出余量；补宽不得超出预算，否则表格宽出容器会重现横向滚动条
    const absorberMin = measureColumnMin(table, lastIdx) + ABSORBER_GUARD
    let budget = containerW - absorberMin
    columns.value.forEach((c, i) => {
      if (i === lastIdx || c.width == null) return
      const need = measureColumnMin(table, i)
      if (c.width >= need) return
      const grow = Math.min(need - c.width, Math.max(0, budget))
      c.width = c.width + grow
      budget -= grow
    })
    if (budget >= 0) settleAbsorber(table, containerW)
  }

  return { columns, onResizeColumn, fitColumns }
}

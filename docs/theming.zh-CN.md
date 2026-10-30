# 写主题

livemark 按主题的话来画文档：编辑器里每一处颜色、字体、长度和盒子都来自
文档的计算样式，而主题就是 CSS。这篇文档是这套 CSS 的参考：读哪些值、
匹配哪些选择器、可以写哪些元素和属性。

内置的样子是 `theme/default.css`，每个主题都叠在它上面，所以主题只写自己
改动的东西。示例主题 `cmd/playground/themes/paper.css` 是一个完整的例子，
playground 把它和它渲染的文档并排放在一起：

```bash
go run ./cmd/playground
```

## 使用主题

```go
import (
	"github.com/CarlOpenLab/livemark"
	"github.com/CarlOpenLab/livemark/theme"
)

sheet, diagnostics := theme.Parse("my-theme", css)
for _, d := range diagnostics {
	log.Print(d) // "12:3: warning: heading has no property ..."
}
ed := livemark.New(src, livemark.Options{Theme: theme.New(sheet)})
```

`theme.Parse` 从不失败：读不懂的 CSS 像浏览器一样被跳过，以诊断报告出来，
带行号和列号。`theme.New` 把表按给出的顺序叠在 `default.css` 上；
`Theme.With` 再加一层，正如应用的设置所做。环境决定深浅色：
`theme.Resolver(theme.Env{Dark: true, Vars: ...})` 就是窗口外观或颜色变化时
编辑器新建的那个，它提供 `--ui-*` 变量。没有窗口、只渲染文档的应用也能用
同样的方式求样式。

## CSS 子集

### 值

- **长度**：`px`、`em`（元素自己的字号）和 `rem`（文档的字号），可以用
  `calc()`、`min()`、`max()`、`clamp()` 和括号组合。`font-size` 和
  `line-height` 还接受百分比；`line-height` 接受数字，是字号的倍数。
- **颜色**：`#rgb`、`#rgba`、`#rrggbb`、`#rrggbbaa`、`rgb()` 和 `rgba()`
  （逗号或空格加 `/` 的透明度）、`hsl()` 和 `hsla()`、颜色名、
  `transparent`、`currentcolor`，以及 `color-mix(in srgb, a 30%, b)`。
- **变量**：在任何元素上声明 `--name: value`，用 `var(--name, fallback)`
  读取。变量按层叠继承；变量里的 `var()` 在声明它的地方求值。窗口提供
  `--ui-text`、`--ui-text-muted`、`--ui-background`、`--ui-surface`、
  `--ui-border`、`--ui-accent`、`--ui-accent-text`、`--ui-selection`、
  `--ui-danger`、`--ui-warning`、`--ui-success`、`--ui-font`、
  `--ui-font-size` 和 `--ui-radius`。
- **字体族**：逗号分隔的名字列表，不是普通单词的名字要加引号；通用族
  `serif`、`sans-serif`、`monospace`、`system-ui`、`ui-serif`、
  `ui-sans-serif`、`ui-monospace`、`ui-rounded`、`cursive` 和 `fantasy`
  选用窗口自己的字体。

### 选择器

选择器用元素的类型、类和属性来点名，组合方式与 CSS 相同：

```css
quote strong { color: #a5392b; }          /* 后代 */
item.checked > paragraph { color: gray; } /* 子元素 */
heading.h1, heading.h2 { font-weight: 700; }
code.block[lang="go"] { /* ... */ }
cell[align="right"] { text-align: right; }
:root { font-family: serif; }
* { }
```

- 类写 `.name`；不支持 id，因为节点没有 id。
- 属性选择器：`[name]`、`[name="value"]`、`~=`、`^=`、`$=`、`*=`，
  值后面可以加 `i` 表示不区分大小写。
- 伪类：`:root`（文档）、`:first-child`、`:last-child`、`:revealed`
  （光标处显示着语法标记）、`:broken`（没加载出来的图片，或没排出来的
  公式）和 `:not(...)`，其中只放一个不带伪元素的复合选择器。
- 伪元素：`::marker` 和 `::checkbox`（列表项）、`::info`（围栏代码块的
  info 串）、`::label`（脚注），以及文档的 `::selection`、`::search-text`
  （选中的那一条带 `:current`）和 `::placeholder`。
- hover、focus、`::before`、`::after` 和 `:nth-child()` 会报告为诊断、
  匹配不到东西：文档的节点没有这些状态，生成的文字也没有地方可放。

### 层叠与 at 规则

主题是一叠表。内置的 `default.css` 是第一层，`theme.New` 给出的层依次
跟在后面。层内按 CSS 的层叠：先比特异性（类、属性和伪类，然后类型和
伪元素），再比顺序，而 `!important` 压过一切不重要的声明。`inherit`、
`initial` 和 `unset` 与 CSS 一样。

- `@media (prefers-color-scheme: dark)` 和 `(prefers-color-scheme: light)`
  选择外观；`and` 可以组合，别的媒体特性不读。
- `@font-face` 引入 TrueType 或 OpenType 字体：`font-family`、`src`、
  `font-weight` 和 `font-style`；`src` 可以列多个文件，取格式支持的
  第一个。
- `@import` 报告后跳过：把规则写进这张表。

每个问题都是诊断：错误是解析器跳过的 CSS，警告是读得懂、在这里不起作用
的 CSS，比如属性用在了它的选择器匹配不到的元素上。playground 把它们
列出来，点一条就跳到那一行。

## 元素

文档的每个节点都是一个元素，名字与 `syntax` 包对节点的称呼相同。主题
可以选择这些：

| 元素 | 是什么 | 类 | 属性 | 伪元素 |
|---|---|---|---|---|
| `document` | 整个文档：`:root` | | | `::selection`、`::search-text`、`::placeholder` |
| `paragraph` | 段落 | | | |
| `heading` | 标题 | `h1`–`h6`、`setext` | `level` | |
| `quote` | 引用 | | | |
| `list` | 列表 | `ordered`、`bullet`、`tight`、`loose` | `start`、`depth` | |
| `item` | 列表项 | `task`、`checked` | `number`、`depth` | `::marker`、`::checkbox` |
| `code` | 行内代码或代码块 | `block`、`fenced`、`indented`、`inline`、`html` | `lang`、`info` | `::info` |
| `math` | 公式 | `block`、`inline` | | |
| `html` | 原始 HTML | `block`、`inline` | `tag` | |
| `table` | 表格 | | | |
| `row` | 表格行 | `header`、`odd`、`even` | | |
| `cell` | 表格单元格 | `header` | `align` | |
| `rule` | 分隔线 | | | |
| `footnote` | 脚注正文 | | `label` | `::label` |
| `front-matter` | YAML front matter | | | |
| `link-def` | 链接引用定义 | | | |
| `strong` | 加粗 | `html` | | |
| `emphasis` | 斜体 | `html` | | |
| `strike` | 删除线 | `html` | | |
| `underline` | 下划线 | | | |
| `mark` | 荧光笔 | | | |
| `sup`、`sub` | 上标、下标 | | | |
| `kbd` | 按键 | | | |
| `link` | 链接 | `auto` | `href`、`title` | |
| `image` | 图片 | `alone` | `src`、`alt`、`title` | |
| `footnote-ref` | 脚注引用 | | `label` | |
| `syntax` | 光标处显示的 Markdown 语法标记 | 见下 | | |
| `token` | 高亮代码的词法单元 | 见下 | | |

`syntax` 的类说明标记属于哪种结构：`strong`、`emphasis`、`strike`、
`code`、`link`、`image`、`math`、`html`、`escape`、`entity`、
`footnote-ref`、`fence`、`info` 和 `rule`。块级语法从不显示：标题的
`#`、围栏和项目符号都被它们的意思取代。行内语法只在光标附近显示，
这时它所在的结构处于 `:revealed` 状态；块的内容在光标里时同样如此。

`token` 是围栏或缩进代码块里、livemark 能高亮的语言的词法单元：
`keyword`、`type`、`function`、`variable`、`property`、`constant`、
`number`、`string`、`escape`、`regexp`、`comment`、`operator`、
`punctuation`、`tag`、`attribute`、`meta`、`heading`、`emphasis`、
`strong`、`link`、`code`、`inserted`、`deleted` 和 `invalid`。它们套在
代码块的 `code` 元素里，所以 `code.block token.keyword` 只给代码块里的
关键字着色。

元素带的类和属性都是节点真有的：只有任务项才有 `task` 和 `checked`，
只有表格的第一行才有 `header`，它后面的行才有 `odd` 和 `even`，只有
段里独一张图片才有 `alone`。任何位置上的路径就是 playground 检查器
显示的那一串，比如 `document › quote › paragraph › strong:revealed`。

## 属性

下面是主题会读的属性，以及初始值、是否继承、用在哪些元素上。值写在
不适用它的元素上会是一条警告，不会悄悄不起作用。

| 属性 | 初始值 | 继承 | 适用 | 说明 |
|---|---|---|---|---|
| `color` | `#000` | 是 | 全部 | |
| `font-family` | `system-ui` | 是 | 全部 | 一个列表 |
| `font-size` | `16px` | 是 | 全部 | 长度或百分比 |
| `font-weight` | `400` | 是 | 全部 | `normal`、`bold` 或 1–1000 |
| `font-style` | `normal` | 是 | 全部 | `normal`、`italic`、`oblique` |
| `font-feature-settings` | `normal` | 是 | 全部 | 加引号的四字母标签 |
| `font-variant-numeric` | `normal` | 是 | 全部 | `normal`、`proportional-nums`、`tabular-nums` |
| `letter-spacing` | `normal` | 是 | 全部 | 长度 |
| `line-height` | `1.75` | 是 | 全部 | 数字、长度或百分比 |
| `text-align` | `start` | 是 | 全部 | `start`、`left`、`center`、`right`、`end` |
| `caret-color` | `auto` | 是 | 全部 | 颜色，或者文字的颜色 |
| `list-style-type` | `auto` | 是 | `list` | `auto`、`disc`、`circle`、`square`、`decimal`、`none`、`"字符串"` |
| `text-decoration-line` | `none` | 否 | 全部 | `underline`、`line-through` |
| `text-decoration-style` | `solid` | 否 | 全部 | `solid`、`wavy` |
| `text-decoration-color` | `currentcolor` | 否 | 全部 | |
| `text-decoration-thickness` | `auto` | 否 | 全部 | 长度或 `auto` |
| `vertical-align` | `baseline` | 否 | 行内 | `baseline`、`super`、`sub`、长度 |
| `opacity` | `1` | 否 | 全部 | 数字或百分比；乘到颜色上 |
| `background-color` | `transparent` | 否 | 全部 | |
| `border-radius` | `0` | 否 | 全部 | |
| `padding-top` … `padding-left` | `0` | 否 | `document`、块、容器 | |
| `margin-top`、`margin-bottom` | `0` | 否 | 块和容器 | |
| `margin-right` | `0` | 否 | 伪元素 | 标记与正文之间 |
| `margin-left` | — | — | 无 | 用 `padding-left` |
| `border-top-width` … `border-left-width` | `medium` | 否 | 块、容器、伪元素、图片 | `thin`、`medium`、`thick` 或长度 |
| `border-top-style` … `border-left-style` | `none` | 否 | 同宽度 | `none`、`solid` |
| `border-top-color` … `border-left-color` | `currentcolor` | 否 | 同宽度 | |
| `max-width` | `820px` | 否 | `document` | 正文那一栏的宽度 |
| `width`、`height` | `auto` | 否 | 块、伪元素 | 分隔线的高度、勾选框的大小 |

简写属性有 `background`、`padding`、`margin`、`border`、`border-top`、
`border-right`、`border-bottom`、`border-left`、`border-width`、
`border-style`、`border-color`、`text-decoration` 和 `list-style`。

### 样式怎么画出来

- `:root` 定下页面：`background-color` 铺满编辑区，`max-width` 和
  `padding` 决定正文那一栏，字体、颜色、`line-height` 和 `caret-color`
  是整篇文档的。
- 块的 `background-color` 和 `border-radius` 填出它的盒子，`padding`
  把内容与盒子的边隔开。
- 边框画块的上边、引用的左边，以及表格的网格（由 `table` 的 `border`
  画出）。其它边读得懂但不画：标题的下划线是 `text-decoration`，不是
  `border-bottom`。
- `text-decoration` 画在文字本身上，所以能跟着标题、链接和段落走；
  颜色和粗细就是上面那几个属性。
- `::marker` 和 `::checkbox` 决定列表标记的大小与颜色，`::info` 是围栏
  代码块的语言名，`::label` 是脚注的 `[^1]`。
- `code` 决定代码的字体，`token.*` 决定代码的颜色，`math` 决定排好的
  公式的墨色。图表用代码块的颜色和 `--diagram-muted`、
  `--diagram-line`、`--diagram-fill`、`--diagram-accent` 以及
  `--ui-background` 这些变量来画。
- `image:broken` 是没加载出来的图片的占位样式，`image:revealed` 是
  光标停在图片源上时的样子。

## 一个完整的例子

`cmd/playground/themes/paper.css` 是示例主题"纸张"：衬线正文、暖色、
印章红、标题下的一条细线、斑马纹表格。开头的一段：

```css
:root {
  --ink: #3a3226;
  --muted: #8a7d66;
  --rule: #ddd0b4;
  --seal: #a5392b;

  font-family: "Songti SC", "Noto Serif CJK SC", "Times New Roman", serif;
  font-size: 16.5px;
  line-height: 1.9;
  color: var(--ink);
  background-color: #faf6ec;
  caret-color: var(--seal);
  max-width: 740px;
  padding: 3rem 44px 0;
}

@media (prefers-color-scheme: dark) {
  :root {
    --ink: #e6ddc9;
    --seal: #d97b6a;
    background-color: #221f1a;
  }
}

heading {
  text-decoration: underline;
  text-decoration-color: var(--rule);
  text-decoration-thickness: 1px;
}
row.even { background-color: rgba(120, 96, 48, .06); }
```

文件余下的部分用同样的办法照顾引用、列表标记、勾选框、链接、代码、
表格和图片占位：几个选择器、上面那些变量，没有别的。

## Playground

`cmd/playground` 是主题作者的工作台：

- 左边的 CSS 编辑器上方有预设：默认（`default.css` 上的一张空表）和
  纸张（示例主题）；输入停下约 150 毫秒后生效。
- 右边的文档覆盖了上面表格里的每一种元素。
- 编辑器下面的栏列出主题的诊断，点一条就跳到它说的那一行。
- 状态栏显示光标处的节点路径，并可以把它作为选择器插入 CSS 编辑器的
  光标处，连规则的括号都备好了。
- 语法树显示文档的公开语法树：每个节点的元素、区间和藏起来的语法。
- 深色切换窗口外观，`@media (prefers-color-scheme)` 随之生效。
- 桌面上，"打开"和"保存"读写主题文件；别的程序改了这个文件时，
  playground 会热重载它。

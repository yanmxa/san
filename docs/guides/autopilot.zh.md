# Autopilot(副驾)

## 概览

Autopilot 是 San 的可配置**副驾**:在固定的几个节点上帮你开 —— 提议或改写
输入、放行灰区工具调用、回答命令的交互问询、替你回答 `AskUserQuestion`、
以及在回合结束后朝着 mission 自动续跑。每个节点是一个可独立开关的
**steer**;默认只开启灰区权限判定这一项。

用 `shift+tab` 切换到 AutoPilot 模式(循环到琥珀色的 `⏵⏵ autopilot on`),
用 `/autopilot` 面板配置。恢复会话(`san -r <id>`)会回到保存时所在的模式。

## 六个 steer

Steer 是按需组合的开关,按自主程度从低到高排列。AutoPilot 模式未开启时,
任何 steer 都不会触发。

| Steer | 默认 | 作用 |
|---|---|---|
| **Suggest** | 关 | 把副驾提议的下一步填进输入提示(幽灵文本)—— 有 mission 时朝 mission 提议,没有则退回通用预测。`tab` 接受、`enter` 发送。只建议、不代发。Suggest *关闭*时,AutoPilot 会整体压掉提示,避免副驾怂恿你。 |
| **Start** | 关 | 负责回合的输入:把你发的每条消息改写成更清晰、贴合 mission 的指令;当你带着 mission、输入框为空进入 AutoPilot 时,自己从 mission 推出第一步并提交,免去开场那一句。 |
| **Permission** | **开** | 自动放行静态规则解不了的灰区工具调用,按可逆性、影响面、数据外泄三轴判断。失败即收紧:任何错误都升级给你。 |
| **Bash** | 关 | 回答已批准命令的交互问询(`Continue? [Y/n]`),仅当回答只是让已批准的动作继续;会扩大范围的一律跳过。 |
| **Question** | 关 | 当 mission 使选择明确且低风险时替你回答 `AskUserQuestion`,否则留给你。选项标签逐字校验 —— 部分或凭空的回答一律转为留给你。 |
| **End** | 关 | 回合结束后判断是否朝 mission 续跑,并自己敲出下一条指令。受 **Continue at most N times** 约束(默认 20);计数在每次人类回合重置。 |

## Mission(任务)

Mission 是副驾本会话要开往的目标 —— 在 `/autopilot` 面板的 Mission 对话框里
口头交代(`enter` 发送、`ctrl+r` 清空、`esc` 存回)。推进类 steer(Suggest、
Start、Question、End)读它;安全类 steer(Permission、Bash)刻意对它不可见,
使动作风险的判断独立于意图。

当 End steer 判定 mission **已完全达成**,会将其退役:清空 mission、steer 归位
到被动基线(Permission + Bash)—— AutoPilot 保持开启,你重新接手,自动放行的
安全网仍在。

## 读懂转录里的标记

| 标记 | 含义 |
|---|---|
| 绿 `↖ autopilot · 2/5` | 上方那条 `❭` 是副驾敲的(第 2 / 共 5 次续跑) |
| 绿 `↖ autopilot · refined` | 上方那条 `❭` 是你的消息,被 Start steer 改写过 |
| 绿 `↳ auto-approved · <理由>` | 判官放行了上方的工具调用 |
| 琥珀 `↳ escalated · <理由>` | 判官把调用退回给你 |
| 绿 `⏵ autopilot · answered for you` | 副驾替你回答了 `AskUserQuestion` |
| 琥珀 `↩ autopilot · this question is yours` | 它把问题留给了你 |
| 琥珀 `↩ autopilot · over to you` | 它停手并交还控制权(判定出错时错误原因缀在后面) |
| 绿 `✓ autopilot · mission complete` | mission 完成并退役 |

判定进行中,模式行显示 `⏵⏵ autopilot · thinking…`;放行计数也在那里
(`· 3 approved · 1 escalated`)。

## 配置

面板编辑的是本会话的实时配置,保存后写入 `settings.json` 作为新会话的默认种子。
会话自身的配置与模式也随转录持久化,`/resume` 时恢复。

```jsonc
{
  "autoPilot": {
    "model": "anthropic/claude-haiku-4-5", // steer 判定用的模型;留空用会话模型
    "systemPrompt": "…",                   // “怎么开” —— 全部 steer 共用
    "systemPromptFile": "~/prompts/pilot.md", // systemPrompt 为空时生效
    "mission": "…",                        // 通常在面板里按会话设置
    "maxContinuations": 20,
    "steers": {
      "suggest": true,
      "turnStart": true,   // Start steer
      "permission": true,  // 省略即默认开;false 则一律升级给你
      "bashPrompt": true,  // Bash steer
      "question": true,
      "turnEnd": true      // End steer
    }
  }
}
```

命名预设:面板的 **▲ Export / ▼ Import** 把整份配置存取于
`~/.san/autopilot/<name>.json`。

## 关联

- [权限模型](../concepts/permission-model.md) —— Permission steer 判定的灰区
  来自这套静态规则;被硬性拦截的动作永远到不了判官面前。
- 判官组件在 `internal/reviewer`(`reviewer.Judge`);steer 与面板在
  `internal/app` / `internal/app/input`。

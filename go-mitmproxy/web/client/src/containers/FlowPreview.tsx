import React from 'react'
import { shallowEqual } from '../utils/utils'
import type { IFlowPreview } from '../utils/flow'

interface IProps {
  flow: IFlowPreview
  isSelected: boolean
  onShowDetail: () => void
}

class FlowPreview extends React.Component<IProps> {
  shouldComponentUpdate(nextProps: IProps) {
    if (nextProps.isSelected === this.props.isSelected && shallowEqual(nextProps.flow, this.props.flow)) {
      return false
    }
    return true
  }

  render() {
    const fp = this.props.flow

    const classNames = []
    if (this.props.isSelected) {
      classNames.push('tr-selected')
    } else if (fp.waitIntercept) {
      classNames.push('tr-wait-intercept')
    } else if (fp.warn) {
      classNames.push('tr-wait-warn')
    }

    return (
      <tr className={classNames.length ? classNames.join(' ') : undefined}
        onClick={() => {
          this.props.onShowDetail()
        }}
      >
        <td>{fp.no}</td>
        <td>{fp.method}</td>
        <td>{fp.host}</td>
        <td>{fp.path}</td>
        <td>{fp.contentType}</td>
        <td>{fp.statusCode}</td>
        <td>{fp.size}</td>
        <td>{fp.costTime}</td>
      </tr>
    )
  }
}

export default FlowPreview

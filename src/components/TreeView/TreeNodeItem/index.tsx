import { useState } from 'react'

import styles from '../styles.module.scss'

export interface TreeNodeData {
    key: {
      [key: string]: string;
    };
    icon: boolean;
    lazy: boolean;
    active: boolean;
    title: string;
    children?: TreeNodeData[];
}

interface TreeNodeItemProps {
    data: TreeNodeData;
    isSelected: boolean;
    onSelect: (data: TreeNodeData) => void
}

export function TreeNodeItem({ data, isSelected, onSelect }: TreeNodeItemProps) {
    const [isExpanded, setIsExpanded] = useState<boolean>(false)

    const toggleExpand = () => setIsExpanded(!isExpanded)

    const handleClick = () => {
        if(data.children && data.children.length > 0) {
            toggleExpand()
        }else {
            onSelect(data)
        }
    }

    return (
        <div className={`${styles.nodeItemContainer} ${isSelected && styles.selected}`}>
            <div
                className={styles.nodeItem}
                onClick={handleClick}
            >
                {data.children && data.children.length > 0 ? (
                    isExpanded ? 
                        <img src='/images/arrow-down.svg' alt='arrow down' /> : 
                        <img src='/images/arrow-right.svg' alt='arrow right' />
                ): null}
                <span>{data.title}</span>
            </div>
            {isExpanded && data.children && (
                <div style={{ paddingLeft: '20px' }}>
                    {data.children.map((child, index) => (
                        <TreeNodeItem
                            key={index}
                            data={child}
                            isSelected={isSelected}
                            onSelect={onSelect}
                        />
                    ))}
                </div>
            )}
        </div>
    )
}
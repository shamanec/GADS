import styles from './styles.module.scss'

export function Modal({ icon, title, block, supporting, children, modalOpen }) {
    return modalOpen ? (
        <div className={styles.modal}>
            <div className={styles.modalContent}>
                <div className={styles.modalHeader}>
                    <div className={styles.header}>
                        <div className={styles.iconContainer}>
                            {icon}
                        </div>
                        <div className={styles.textAndSupportingText}>
                            <h2>{title} {block}</h2>
                            <span>{supporting}</span>
                        </div>
                    </div>
                    <div className={styles.body}>
                        <div className={styles.content}>
                            {children}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    ) : null
}